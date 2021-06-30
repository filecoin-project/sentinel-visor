package storage

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/go-pg/pg/v10/types"
	"github.com/go-pg/pgext"
	logging "github.com/ipfs/go-log/v2"
	"github.com/raulk/clock"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/model"
	"github.com/filecoin-project/sentinel-visor/model/actors/common"
	init_ "github.com/filecoin-project/sentinel-visor/model/actors/init_"
	"github.com/filecoin-project/sentinel-visor/model/actors/market"
	"github.com/filecoin-project/sentinel-visor/model/actors/miner"
	"github.com/filecoin-project/sentinel-visor/model/actors/multisig"
	"github.com/filecoin-project/sentinel-visor/model/actors/power"
	"github.com/filecoin-project/sentinel-visor/model/actors/reward"
	"github.com/filecoin-project/sentinel-visor/model/blocks"
	"github.com/filecoin-project/sentinel-visor/model/chain"
	"github.com/filecoin-project/sentinel-visor/model/derived"
	"github.com/filecoin-project/sentinel-visor/model/messages"
	"github.com/filecoin-project/sentinel-visor/model/msapprovals"
)

var models = []interface{}{
	(*blocks.BlockHeader)(nil),
	(*blocks.BlockParent)(nil),
	(*blocks.DrandBlockEntrie)(nil),

	(*miner.MinerSectorDeal)(nil),
	(*miner.MinerSectorInfo)(nil),
	(*miner.MinerSectorPost)(nil),
	(*miner.MinerPreCommitInfo)(nil),
	(*miner.MinerSectorEvent)(nil),
	(*miner.MinerCurrentDeadlineInfo)(nil),
	(*miner.MinerFeeDebt)(nil),
	(*miner.MinerLockedFund)(nil),
	(*miner.MinerInfo)(nil),

	(*market.MarketDealProposal)(nil),
	(*market.MarketDealState)(nil),

	(*messages.Message)(nil),
	(*messages.BlockMessage)(nil),
	(*messages.Receipt)(nil),
	(*messages.MessageGasEconomy)(nil),
	(*messages.ParsedMessage)(nil),

	(*multisig.MultisigTransaction)(nil),

	(*power.ChainPower)(nil),
	(*power.PowerActorClaim)(nil),
	(*reward.ChainReward)(nil),
	(*common.Actor)(nil),
	(*common.ActorState)(nil),

	(*init_.IdAddress)(nil),

	(*derived.GasOutputs)(nil),
	(*chain.ChainEconomics)(nil),
	(*msapprovals.MultisigApproval)(nil),
}

var log = logging.Logger("visor/storage")

// Advisory locks
var (
	SchemaLock AdvisoryLock = 1
)

var (
	ErrSchemaTooOld = errors.New("database schema is too old and requires migration")
	ErrSchemaTooNew = errors.New("database schema is too new for this version of visor")
	ErrNameTooLong  = errors.New("name exceeds maximum length for postgres application names")
)

const MaxPostgresNameLength = 64

func NewDatabase(ctx context.Context, url string, poolSize int, name string, schemaName string, upsert bool) (*Database, error) {
	if len(name) > MaxPostgresNameLength {
		return nil, ErrNameTooLong
	}

	opt, err := pg.ParseURL(url)
	if err != nil {
		return nil, xerrors.Errorf("parse database URL: %w", err)
	}
	opt.PoolSize = poolSize
	if opt.ApplicationName == "" {
		opt.ApplicationName = name
	}

	onConnect := func(ctx context.Context, conn *pg.Conn) error {
		_, err := conn.Exec("set search_path=?", schemaName)
		if err != nil {
			log.Errorf("failed to set postgresql search_path: %v", err)
		}
		return nil
	}

	if opt.OnConnect == nil {
		opt.OnConnect = onConnect
	} else {
		// Chain functions
		prevOnConnect := opt.OnConnect
		opt.OnConnect = func(ctx context.Context, conn *pg.Conn) error {
			if err := prevOnConnect(ctx, conn); err != nil {
				return err
			}
			return onConnect(ctx, conn)
		}
	}

	return &Database{
		opt:        opt,
		schemaName: schemaName,
		Clock:      clock.New(),
		Upsert:     upsert,
	}, nil
}

func NewDatabaseFromDB(ctx context.Context, db *pg.DB, schemaName string) (*Database, error) {
	dbVersion, err := validateDatabaseSchemaVersion(ctx, db, schemaName)
	if err != nil {
		return nil, err
	}

	return &Database{
		db:      db,
		opt:     new(pg.Options),
		Clock:   clock.New(),
		version: dbVersion,
	}, nil
}

var _ Connector = (*Database)(nil)

type Database struct {
	db         *pg.DB
	opt        *pg.Options
	schemaName string
	Clock      clock.Clock
	Upsert     bool
	version    model.Version // schema version identified in the database
}

// Connect opens a connection to the database and checks that the schema is compatible with the version required
// by this version of visor. ErrSchemaTooOld is returned if the database schema is older than the current schema,
// ErrSchemaTooNew if it is newer.
func (d *Database) Connect(ctx context.Context) error {
	db, err := connect(ctx, d.opt)
	if err != nil {
		return xerrors.Errorf("connect: %w", err)
	}

	dbVersion, err := validateDatabaseSchemaVersion(ctx, db, d.schemaName)
	if err != nil {
		_ = db.Close() // nolint: errcheck
		return err
	}

	d.db = db
	d.version = dbVersion

	return nil
}

func connect(ctx context.Context, opt *pg.Options) (*pg.DB, error) {
	db := pg.Connect(opt)
	db = db.WithContext(ctx)
	db.AddQueryHook(&pgext.OpenTelemetryHook{})

	// Check if connection credentials are valid and PostgreSQL is up and running.
	if err := db.Ping(ctx); err != nil {
		return nil, xerrors.Errorf("ping database: %w", err)
	}

	// Acquire a shared lock on the schema to notify other instances that we are running
	if err := SchemaLock.LockShared(ctx, db); err != nil {
		_ = db.Close() // nolint: errcheck
		return nil, xerrors.Errorf("failed to acquire schema lock, possible migration in progress: %w", err)
	}

	return db, nil
}

func (d *Database) IsConnected(ctx context.Context) bool {
	if d.db == nil {
		return false
	}

	if err := d.db.Ping(ctx); err != nil {
		return false
	}

	return true
}

func (d *Database) Close(ctx context.Context) error {
	// Advisory locks are automatically closed at end of session but its still good practice to close explicitly
	if err := SchemaLock.UnlockShared(ctx, d.db); err != nil && !errors.Is(err, context.Canceled) {
		log.Errorf("failed to release schema lock: %v", err)
	}

	err := d.db.Close()
	d.db = nil
	return err
}

// VerifyCurrentSchema compares the schema present in the database with the models used by visor
// and returns an error if they are incompatible
func (d *Database) VerifyCurrentSchema(ctx context.Context) error {
	// If we're already connected then use that connection
	if d.db != nil {
		return verifyCurrentSchema(ctx, d.db, d.schemaName)
	}

	// Temporarily connect
	db, err := connect(ctx, d.opt)
	if err != nil {
		return xerrors.Errorf("connect: %w", err)
	}
	defer db.Close() // nolint: errcheck
	return verifyCurrentSchema(ctx, db, d.schemaName)
}

func verifyCurrentSchema(ctx context.Context, db *pg.DB, schemaName string) error {
	type versionable interface {
		AsVersion(model.Version) (interface{}, bool)
	}

	version, initialized, err := getDatabaseSchemaVersion(ctx, db, schemaName)
	if err != nil {
		return xerrors.Errorf("get schema version: %w", err)
	}

	if !initialized {
		return xerrors.Errorf("schema not installed in database")
	}

	valid := true
	for _, model := range models {
		if vm, ok := model.(versionable); ok {
			m, ok := vm.AsVersion(version)
			if !ok {
				return xerrors.Errorf("model %T does not support version %s", model, version)
			}
			model = m
		}

		q := db.Model(model)
		tm := q.TableModel()
		m := tm.Table()
		err := verifyModel(ctx, db, schemaName, m)
		if err != nil {
			valid = false
			log.Errorf("verify schema: %v", err)
		}

	}
	if !valid {
		return xerrors.Errorf("database schema was not compatible with current models")
	}
	return nil
}

func verifyModel(ctx context.Context, db *pg.DB, schemaName string, m *orm.Table) error {
	tableName := stripQuotes(m.SQLNameForSelects)

	exists, err := tableExists(ctx, db, schemaName, tableName)
	if err != nil {
		return xerrors.Errorf("querying table: %v", err)
	}
	if !exists {
		return xerrors.Errorf("required table %s not found", m.SQLName)
	}

	for _, fld := range m.Fields {
		var datatype string
		_, err := db.QueryOne(pg.Scan(&datatype), `SELECT data_type FROM information_schema.columns WHERE table_schema=? AND table_name=? AND column_name=?`, schemaName, tableName, fld.SQLName)
		if err != nil {
			if errors.Is(err, pg.ErrNoRows) {
				return xerrors.Errorf("required column %s.%s not found", tableName, fld.SQLName)
			}
			return xerrors.Errorf("querying field: %v %T", err, err)
		}
		if datatype == "USER-DEFINED" {
			_, err := db.QueryOne(pg.Scan(&datatype), `SELECT udt_name FROM information_schema.columns WHERE table_schema=? AND table_name=? AND column_name=?`, schemaName, tableName, fld.SQLName)
			if err != nil {
				if errors.Is(err, pg.ErrNoRows) {
					return xerrors.Errorf("required column %s.%s not found", tableName, fld.SQLName)
				}
				return xerrors.Errorf("querying field: %v %T", err, err)
			}
		}

		// Some common aliases
		if datatype == "timestamp with time zone" {
			datatype = "timestamptz"
		} else if datatype == "timestamp without time zone" {
			datatype = "timestamp"
		}

		if datatype != fld.SQLType {
			return xerrors.Errorf("column %s.%s had datatype %s, expected %s", tableName, fld.SQLName, datatype, fld.SQLType)
		}

	}

	return nil
}

func tableExists(ctx context.Context, db *pg.DB, schemaName string, tableName string) (bool, error) {
	var exists bool
	_, err := db.QueryOneContext(ctx, pg.Scan(&exists), `SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema=? AND table_name=?)`, schemaName, tableName)
	if err != nil {
		return false, xerrors.Errorf("querying table: %v", err)
	}

	return exists, nil
}

func stripQuotes(s types.Safe) string {
	return strings.Trim(string(s), `"`)
}

// PersistBatch persists a batch of persistables in a single transaction
func (d *Database) PersistBatch(ctx context.Context, ps ...model.Persistable) error {
	return d.db.RunInTransaction(ctx, func(tx *pg.Tx) error {
		txs := &TxStorage{
			tx:     tx,
			upsert: d.Upsert,
		}

		for _, p := range ps {
			if err := p.Persist(ctx, txs, d.version); err != nil {
				return err
			}
		}

		return nil
	})
}

func (d *Database) ExecContext(c context.Context, query interface{}, params ...interface{}) (pg.Result, error) {
	return d.db.ExecContext(c, query, params...)
}

type TxStorage struct {
	tx     *pg.Tx
	upsert bool
}

// PersistModel persists a single model
func (s *TxStorage) PersistModel(ctx context.Context, m interface{}) error {
	value := reflect.ValueOf(m)

	elemKind := value.Kind()
	if value.Kind() == reflect.Ptr {
		elemKind = value.Elem().Kind()
	}

	if elemKind == reflect.Slice || elemKind == reflect.Array {
		// Avoid persisting zero length lists
		if value.Len() == 0 {
			return nil
		}

		// go-pg expects pointers to slices. We can fix it up.
		if value.Kind() != reflect.Ptr {
			p := reflect.New(value.Type())
			p.Elem().Set(value)
			m = p.Interface()
		}

	}
	if s.upsert {
		conflict, upsert := GenerateUpsertStrings(m)
		if _, err := s.tx.ModelContext(ctx, m).
			OnConflict(conflict).
			Set(upsert).
			Insert(); err != nil {
			return xerrors.Errorf("upserting model: %w", err)
		}
	} else {
		if _, err := s.tx.ModelContext(ctx, m).
			OnConflict("do nothing").
			Insert(); err != nil {
			return xerrors.Errorf("persisting model: %w", err)
		}
	}
	return nil
}

// GenerateUpsertString accepts a visor model and returns two string containing SQL that may be used
// to upsert the model. The first string is the conflict statement and the second is the insert.
//
// Example given the below model:
//
// type SomeModel struct {
// 	Height    int64  `pg:",pk,notnull,use_zero"`
// 	MinerID   string `pg:",pk,notnull"`
// 	StateRoot string `pg:",pk,notnull"`
// 	OwnerID  string `pg:",notnull"`
// 	WorkerID string `pg:",notnull"`
// }
//
// The strings returned are:
// conflict string:
//	"(cid, height, state_root) DO UPDATE"
// update string:
// 	"owner_id" = EXCLUDED.owner_id, "worker_id" = EXCLUDED.worker_id
func GenerateUpsertStrings(model interface{}) (string, string) {
	var cf []string
	var ucf []string

	// gather all public keys
	for _, pk := range pg.Model(model).TableModel().Table().PKs {
		cf = append(cf, pk.SQLName)
	}
	// gather all other fields
	for _, field := range pg.Model(model).TableModel().Table().DataFields {
		ucf = append(ucf, field.SQLName)
	}

	// consistent ordering in sql statements.
	sort.Strings(cf)
	sort.Strings(ucf)

	// build the conflict string
	var conflict strings.Builder
	conflict.WriteString("(")
	for i, str := range cf {
		conflict.WriteString(str)
		// if this isn't the last field in the conflict statement add a comma.
		if !(i == len(cf)-1) {
			conflict.WriteString(", ")
		}
	}
	conflict.WriteString(") DO UPDATE")

	// build the upsert string
	var upsert strings.Builder
	for i, str := range ucf {
		upsert.WriteString("\"" + str + "\"" + " = EXCLUDED." + str)
		// if this isn't the last field in the upsert statement add a comma.
		if !(i == len(ucf)-1) {
			upsert.WriteString(", ")
		}
	}
	return conflict.String(), upsert.String()
}

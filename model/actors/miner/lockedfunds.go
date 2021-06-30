package miner

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

func init() {
	registry.ModelRegistry.Register(registry.ActorStatesMinerTask, &MinerLockedFund{})
}

type MinerLockedFund struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	MinerID   string `pg:",pk,notnull"`
	StateRoot string `pg:",pk,notnull"`

	LockedFunds       string `pg:"type:numeric,notnull"`
	InitialPledge     string `pg:"type:numeric,notnull"`
	PreCommitDeposits string `pg:"type:numeric,notnull"`
}

type MinerLockedFundV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"miner_locked_funds"`
	Height    int64    `pg:",pk,notnull,use_zero"`
	MinerID   string   `pg:",pk,notnull"`
	StateRoot string   `pg:",pk,notnull"`

	LockedFunds       string `pg:",notnull"`
	InitialPledge     string `pg:",notnull"`
	PreCommitDeposits string `pg:",notnull"`
}

func (m *MinerLockedFund) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if m == nil {
			return (*MinerLockedFundV0)(nil), true
		}

		return &MinerLockedFundV0{
			Height:            m.Height,
			MinerID:           m.MinerID,
			StateRoot:         m.StateRoot,
			LockedFunds:       m.LockedFunds,
			InitialPledge:     m.InitialPledge,
			PreCommitDeposits: m.PreCommitDeposits,
		}, true
	case 1:
		return m, true
	default:
		return nil, false
	}
}

func (m *MinerLockedFund) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerLockedFund.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_locked_funds"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vm, ok := m.AsVersion(version)
	if !ok {
		return xerrors.Errorf("MinerLockedFund not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vm)
}

type MinerLockedFundsList []*MinerLockedFund

func (ml MinerLockedFundsList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerLockedFundsList.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_locked_funds"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if len(ml) == 0 {
		return nil
	}

	if version.Major != 1 {
		// Support older versions, but in a non-optimal way
		for _, m := range ml {
			if err := m.Persist(ctx, s, version); err != nil {
				return err
			}
		}
		return nil
	}

	return s.PersistModel(ctx, ml)
}

package power

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

func init() {
	registry.ModelRegistry.Register(registry.ActorStatesPowerTask, &ChainPower{})
}

type ChainPower struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	StateRoot string `pg:",pk"`

	TotalRawBytesPower string `pg:"type:numeric,notnull"`
	TotalQABytesPower  string `pg:"type:numeric,notnull"`

	TotalRawBytesCommitted string `pg:"type:numeric,notnull"`
	TotalQABytesCommitted  string `pg:"type:numeric,notnull"`

	TotalPledgeCollateral string `pg:"type:numeric,notnull"`

	QASmoothedPositionEstimate string `pg:"type:numeric,notnull"`
	QASmoothedVelocityEstimate string `pg:"type:numeric,notnull"`

	MinerCount              uint64 `pg:",use_zero"`
	ParticipatingMinerCount uint64 `pg:",use_zero"`
}

type ChainPowerV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"chain_powers"`
	Height    int64    `pg:",pk,notnull,use_zero"`
	StateRoot string   `pg:",pk"`

	TotalRawBytesPower string `pg:",notnull"`
	TotalQABytesPower  string `pg:",notnull"`

	TotalRawBytesCommitted string `pg:",notnull"`
	TotalQABytesCommitted  string `pg:",notnull"`

	TotalPledgeCollateral string `pg:",notnull"`

	QASmoothedPositionEstimate string `pg:",notnull"`
	QASmoothedVelocityEstimate string `pg:",notnull"`

	MinerCount              uint64 `pg:",use_zero"`
	ParticipatingMinerCount uint64 `pg:",use_zero"`
}

func (cp *ChainPower) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if cp == nil {
			return (*ChainPowerV0)(nil), true
		}

		return &ChainPowerV0{
			Height:                     cp.Height,
			StateRoot:                  cp.StateRoot,
			TotalRawBytesPower:         cp.TotalRawBytesPower,
			TotalQABytesPower:          cp.TotalQABytesPower,
			TotalRawBytesCommitted:     cp.TotalRawBytesCommitted,
			TotalQABytesCommitted:      cp.TotalQABytesCommitted,
			TotalPledgeCollateral:      cp.TotalPledgeCollateral,
			QASmoothedPositionEstimate: cp.QASmoothedPositionEstimate,
			QASmoothedVelocityEstimate: cp.QASmoothedVelocityEstimate,
			MinerCount:                 cp.MinerCount,
			ParticipatingMinerCount:    cp.ParticipatingMinerCount,
		}, true
	case 1:
		return cp, true
	default:
		return nil, false
	}
}

func (cp *ChainPower) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "ChainPower.PersistWithTx")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "chain_powers"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vcp, ok := cp.AsVersion(version)
	if !ok {
		return xerrors.Errorf("ChainPower not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vcp)
}

// ChainPowerList is a slice of ChainPowers for batch insertion.
type ChainPowerList []*ChainPower

// PersistWithTx makes a batch insertion of the list using the given
// transaction.
func (cpl ChainPowerList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "ChainPowerList.PersistWithTx", trace.WithAttributes(label.Int("count", len(cpl))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "chain_powers"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if len(cpl) == 0 {
		return nil
	}

	if version.Major != 1 {
		// Support older versions, but in a non-optimal way
		for _, m := range cpl {
			if err := m.Persist(ctx, s, version); err != nil {
				return err
			}
		}
		return nil
	}

	return s.PersistModel(ctx, cpl)
}

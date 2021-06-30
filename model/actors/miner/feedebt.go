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
	registry.ModelRegistry.Register(registry.ActorStatesMinerTask, &MinerFeeDebt{})
}

type MinerFeeDebt struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	MinerID   string `pg:",pk,notnull"`
	StateRoot string `pg:",pk,notnull"`

	FeeDebt string `pg:"type:numeric,notnull"`
}

type MinerFeeDebtV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"miner_fee_debts"`
	Height    int64    `pg:",pk,notnull,use_zero"`
	MinerID   string   `pg:",pk,notnull"`
	StateRoot string   `pg:",pk,notnull"`

	FeeDebt string `pg:",notnull"`
}

func (m *MinerFeeDebt) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if m == nil {
			return (*MinerFeeDebtV0)(nil), true
		}

		return &MinerFeeDebtV0{
			Height:    m.Height,
			MinerID:   m.MinerID,
			StateRoot: m.StateRoot,
			FeeDebt:   m.FeeDebt,
		}, true
	case 1:
		return m, true
	default:
		return nil, false
	}
}

func (m *MinerFeeDebt) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerFeeDebt.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_fee_debts"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vm, ok := m.AsVersion(version)
	if !ok {
		return xerrors.Errorf("MinerFeeDebt not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vm)
}

type MinerFeeDebtList []*MinerFeeDebt

func (ml MinerFeeDebtList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerFeeDebtList.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_fee_debts"))
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

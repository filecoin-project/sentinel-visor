package miner

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
	registry.ModelRegistry.Register(registry.ActorStatesMinerTask, &MinerSectorInfo{})
}

type MinerSectorInfo struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	MinerID   string `pg:",pk,notnull"`
	SectorID  uint64 `pg:",pk,use_zero"`
	StateRoot string `pg:",pk,notnull"`

	SealedCID string `pg:",notnull"`

	ActivationEpoch int64 `pg:",use_zero"`
	ExpirationEpoch int64 `pg:",use_zero"`

	DealWeight         string `pg:"type:numeric,notnull"`
	VerifiedDealWeight string `pg:"type:numeric,notnull"`

	InitialPledge         string `pg:"type:numeric,notnull"`
	ExpectedDayReward     string `pg:"type:numeric,notnull"`
	ExpectedStoragePledge string `pg:"type:numeric,notnull"`
}

type MinerSectorInfoV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"miner_sector_infos"`
	Height    int64    `pg:",pk,notnull,use_zero"`
	MinerID   string   `pg:",pk,notnull"`
	SectorID  uint64   `pg:",pk,use_zero"`
	StateRoot string   `pg:",pk,notnull"`

	SealedCID string `pg:",notnull"`

	ActivationEpoch int64 `pg:",use_zero"`
	ExpirationEpoch int64 `pg:",use_zero"`

	DealWeight         string `pg:",notnull"`
	VerifiedDealWeight string `pg:",notnull"`

	InitialPledge         string `pg:",notnull"`
	ExpectedDayReward     string `pg:",notnull"`
	ExpectedStoragePledge string `pg:",notnull"`
}

func (msi *MinerSectorInfo) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if msi == nil {
			return (*MinerSectorInfoV0)(nil), true
		}

		return &MinerSectorInfoV0{
			Height:                msi.Height,
			MinerID:               msi.MinerID,
			SectorID:              msi.SectorID,
			StateRoot:             msi.StateRoot,
			SealedCID:             msi.SealedCID,
			ActivationEpoch:       msi.ActivationEpoch,
			ExpirationEpoch:       msi.ExpirationEpoch,
			DealWeight:            msi.DealWeight,
			VerifiedDealWeight:    msi.VerifiedDealWeight,
			InitialPledge:         msi.InitialPledge,
			ExpectedDayReward:     msi.ExpectedDayReward,
			ExpectedStoragePledge: msi.ExpectedStoragePledge,
		}, true
	case 1:
		return msi, true
	default:
		return nil, false
	}
}

func (msi *MinerSectorInfo) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_sector_infos"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	m, ok := msi.AsVersion(version)
	if !ok {
		return xerrors.Errorf("MinerSectorInfo not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, m)
}

type (
	MinerSectorInfoList []*MinerSectorInfo
)

func (ml MinerSectorInfoList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerSectorInfoList.Persist", trace.WithAttributes(label.Int("count", len(ml))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_sector_infos"))
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

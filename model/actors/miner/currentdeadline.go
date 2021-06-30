package miner

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

func init() {
	registry.ModelRegistry.Register(registry.ActorStatesMinerTask, &MinerCurrentDeadlineInfo{})
}

type MinerCurrentDeadlineInfo struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	MinerID   string `pg:",pk,notnull"`
	StateRoot string `pg:",pk,notnull"`

	DeadlineIndex uint64 `pg:",notnull,use_zero"`
	PeriodStart   int64  `pg:",notnull,use_zero"`
	Open          int64  `pg:",notnull,use_zero"`
	Close         int64  `pg:",notnull,use_zero"`
	Challenge     int64  `pg:",notnull,use_zero"`
	FaultCutoff   int64  `pg:",notnull,use_zero"`
}

func (m *MinerCurrentDeadlineInfo) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerCurrentDeadlineInfo.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_current_deadline_infos"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, m)
}

type MinerCurrentDeadlineInfoList []*MinerCurrentDeadlineInfo

func (ml MinerCurrentDeadlineInfoList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerCurrentDeadlineInfoList.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_current_deadline_infos"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if len(ml) == 0 {
		return nil
	}
	return s.PersistModel(ctx, ml)
}

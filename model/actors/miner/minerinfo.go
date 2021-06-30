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
	registry.ModelRegistry.Register(registry.ActorStatesMinerTask, &MinerInfo{})
}

type MinerInfo struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	MinerID   string `pg:",pk,notnull"`
	StateRoot string `pg:",pk,notnull"`

	OwnerID  string `pg:",notnull"`
	WorkerID string `pg:",notnull"`

	NewWorker         string
	WorkerChangeEpoch int64 `pg:",notnull,use_zero"`

	ConsensusFaultedElapsed int64 `pg:",notnull,use_zero"`

	PeerID           string
	ControlAddresses []string
	MultiAddresses   []string

	SectorSize uint64 `pg:",notnull,use_zero"`
}

func (m *MinerInfo) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerInfoModel.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_infos"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, m)
}

type MinerInfoList []*MinerInfo

func (ml MinerInfoList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerInfoList.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_infos"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if len(ml) == 0 {
		return nil
	}
	return s.PersistModel(ctx, ml)
}

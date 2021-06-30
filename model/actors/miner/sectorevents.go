package miner

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

const (
	PreCommitAdded   = "PRECOMMIT_ADDED"
	PreCommitExpired = "PRECOMMIT_EXPIRED"

	CommitCapacityAdded = "COMMIT_CAPACITY_ADDED"

	SectorAdded      = "SECTOR_ADDED"
	SectorExtended   = "SECTOR_EXTENDED"
	SectorFaulted    = "SECTOR_FAULTED"
	SectorRecovering = "SECTOR_RECOVERING"
	SectorRecovered  = "SECTOR_RECOVERED"

	SectorExpired    = "SECTOR_EXPIRED"
	SectorTerminated = "SECTOR_TERMINATED"
)

func init() {
	registry.ModelRegistry.Register(registry.ActorStatesMinerTask, &MinerSectorEvent{})
}

type MinerSectorEvent struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	MinerID   string `pg:",pk,notnull"`
	SectorID  uint64 `pg:",pk,use_zero"`
	StateRoot string `pg:",pk,notnull"`

	// https://github.com/go-pg/pg/issues/993
	// override the SQL type with enum type, see 1_chainwatch.go for enum definition
	//lint:ignore SA5008 duplicate tag allowed by go-pg
	Event string `pg:"type:miner_sector_event_type" pg:",pk,notnull"`
}

func (mse *MinerSectorEvent) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_sector_events"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, mse)
}

type MinerSectorEventList []*MinerSectorEvent

func (l MinerSectorEventList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MinerSectorEventList.Persist", trace.WithAttributes(label.Int("count", len(l))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "miner_sector_events"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if len(l) == 0 {
		return nil
	}

	return s.PersistModel(ctx, l)
}

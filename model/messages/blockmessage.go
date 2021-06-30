package messages

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

func init() {
	registry.ModelRegistry.Register(registry.MessagesTask, &BlockMessage{})
}

type BlockMessage struct {
	Height  int64  `pg:",pk,notnull,use_zero"`
	Block   string `pg:",pk,notnull"`
	Message string `pg:",pk,notnull"`
}

func (bm *BlockMessage) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "block_messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, bm)
}

type BlockMessages []*BlockMessage

func (bms BlockMessages) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(bms) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "BlockMessages.Persist", trace.WithAttributes(label.Int("count", len(bms))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "block_messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, bms)
}

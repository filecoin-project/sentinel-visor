package blocks

import (
	"context"

	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

func init() {
	registry.ModelRegistry.Register(registry.BlocksTask, &BlockParent{})
}

type BlockParent struct {
	Height int64  `pg:",pk,notnull,use_zero"`
	Block  string `pg:",pk,notnull"`
	Parent string `pg:",notnull"`
}

func (bp *BlockParent) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "block_parents"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, bp)
}

type BlockParents []*BlockParent

func NewBlockParents(header *types.BlockHeader) BlockParents {
	var out BlockParents
	for _, p := range header.Parents {
		out = append(out, &BlockParent{
			Height: int64(header.Height),
			Block:  header.Cid().String(),
			Parent: p.String(),
		})
	}
	return out
}

func (bps BlockParents) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(bps) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "BlockParents.Persist", trace.WithAttributes(label.Int("count", len(bps))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "block_parents"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, bps)
}

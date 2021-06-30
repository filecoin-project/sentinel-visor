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

func NewDrandBlockEntries(header *types.BlockHeader) DrandBlockEntries {
	var out DrandBlockEntries
	for _, ent := range header.BeaconEntries {
		out = append(out, &DrandBlockEntrie{
			Round: ent.Round,
			Block: header.Cid().String(),
		})
	}
	return out
}

func init() {
	registry.ModelRegistry.Register(registry.BlocksTask, &DrandBlockEntrie{})
}

type DrandBlockEntrie struct {
	Round uint64 `pg:",pk,use_zero"`
	Block string `pg:",notnull"`
}

func (dbe *DrandBlockEntrie) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "drand_block_entries"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, dbe)
}

type DrandBlockEntries []*DrandBlockEntrie

func (dbes DrandBlockEntries) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(dbes) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "DrandBlockEntries.Persist", trace.WithAttributes(label.Int("count", len(dbes))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "drand_block_entries"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, dbes)
}

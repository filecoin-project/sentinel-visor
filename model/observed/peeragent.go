package observed

import (
	"context"
	"time"

	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

type PeerAgent struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"observed_peer_agents"`

	// ObservedAt is the time the observation was made
	ObservedAt time.Time `pg:",notnull"`

	// RawAgent is the raw peer agent string
	RawAgent string `pg:",notnull"`

	// NormalizedAgent is a parsed version of peer agent string, stripping out patch versions
	NormalizedAgent string `pg:",notnull"`

	// Count is the number of peers with the associated agent
	Count int64 `pg:",use_zero,notnull"`
}

func (p *PeerAgent) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "observed_peer_agents"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, p)
}

type PeerAgentList []*PeerAgent

func (l PeerAgentList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(l) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "PeerAgentList.Persist", trace.WithAttributes(label.Int("count", len(l))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "observed_peer_agents"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, l)
}

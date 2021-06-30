package market

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
	registry.ModelRegistry.Register(registry.ActorStatesMarketTask, &MarketDealProposal{})
}

type MarketDealProposal struct {
	Height    int64  `pg:",pk,notnull,use_zero"`
	DealID    uint64 `pg:",pk,use_zero"`
	StateRoot string `pg:",notnull"`

	PaddedPieceSize   uint64 `pg:",use_zero"`
	UnpaddedPieceSize uint64 `pg:",use_zero"`

	StartEpoch int64 `pg:",use_zero"`
	EndEpoch   int64 `pg:",use_zero"`

	ClientID             string `pg:",notnull"`
	ProviderID           string `pg:",notnull"`
	ClientCollateral     string `pg:",notnull"`
	ProviderCollateral   string `pg:",notnull"`
	StoragePricePerEpoch string `pg:",notnull"`
	PieceCID             string `pg:",notnull"`

	IsVerified bool `pg:",notnull,use_zero"`
	Label      string
}

func (dp *MarketDealProposal) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "market_deal_proposals"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, dp)
}

type MarketDealProposals []*MarketDealProposal

func (dps MarketDealProposals) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "MarketDealProposals.Persist", trace.WithAttributes(label.Int("count", len(dps))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "market_deal_proposals"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, dps)
}

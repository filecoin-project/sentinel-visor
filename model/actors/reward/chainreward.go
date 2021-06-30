package reward

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
	registry.ModelRegistry.Register(registry.ActorStatesRewardTask, &ChainReward{})
}

type ChainReward struct {
	Height                            int64  `pg:",pk,notnull,use_zero"`
	StateRoot                         string `pg:",pk,notnull"`
	CumSumBaseline                    string `pg:"type:numeric,notnull"`
	CumSumRealized                    string `pg:"type:numeric,notnull"`
	EffectiveBaselinePower            string `pg:"type:numeric,notnull"`
	NewBaselinePower                  string `pg:"type:numeric,notnull"`
	NewRewardSmoothedPositionEstimate string `pg:"type:numeric,notnull"`
	NewRewardSmoothedVelocityEstimate string `pg:"type:numeric,notnull"`
	TotalMinedReward                  string `pg:"type:numeric,notnull"`
	NewReward                         string `pg:"type:numeric,notnull"`
	EffectiveNetworkTime              int64  `pg:",use_zero"`
}

type ChainRewardV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName                         struct{} `pg:"chain_rewards"`
	Height                            int64    `pg:",pk,notnull,use_zero"`
	StateRoot                         string   `pg:",pk,notnull"`
	CumSumBaseline                    string   `pg:",notnull"`
	CumSumRealized                    string   `pg:",notnull"`
	EffectiveBaselinePower            string   `pg:",notnull"`
	NewBaselinePower                  string   `pg:",notnull"`
	NewRewardSmoothedPositionEstimate string   `pg:",notnull"`
	NewRewardSmoothedVelocityEstimate string   `pg:",notnull"`
	TotalMinedReward                  string   `pg:",notnull"`

	NewReward            string `pg:",use_zero"`
	EffectiveNetworkTime int64  `pg:",use_zero"`
}

func (r *ChainReward) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if r == nil {
			return (*ChainRewardV0)(nil), true
		}

		return &ChainRewardV0{
			Height:                            r.Height,
			StateRoot:                         r.StateRoot,
			CumSumBaseline:                    r.CumSumBaseline,
			CumSumRealized:                    r.CumSumRealized,
			EffectiveBaselinePower:            r.EffectiveBaselinePower,
			NewBaselinePower:                  r.NewBaselinePower,
			NewRewardSmoothedPositionEstimate: r.NewRewardSmoothedPositionEstimate,
			NewRewardSmoothedVelocityEstimate: r.NewRewardSmoothedVelocityEstimate,
			TotalMinedReward:                  r.TotalMinedReward,
			NewReward:                         r.NewReward,
			EffectiveNetworkTime:              r.EffectiveNetworkTime,
		}, true
	case 1:
		return r, true
	default:
		return nil, false
	}
}

func (r *ChainReward) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "ChainReward.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "chain_rewards"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vr, ok := r.AsVersion(version)
	if !ok {
		return xerrors.Errorf("ChainReward not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vr)
}

package power

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
	registry.ModelRegistry.Register(registry.ActorStatesPowerTask, &PowerActorClaim{})
}

type PowerActorClaim struct {
	Height          int64  `pg:",pk,notnull,use_zero"`
	MinerID         string `pg:",pk,notnull"`
	StateRoot       string `pg:",pk,notnull"`
	RawBytePower    string `pg:"type:numeric,notnull"`
	QualityAdjPower string `pg:"type:numeric,notnull"`
}

type PowerActorClaimV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName       struct{} `pg:"power_actor_claims"`
	Height          int64    `pg:",pk,notnull,use_zero"`
	MinerID         string   `pg:",pk,notnull"`
	StateRoot       string   `pg:",pk,notnull"`
	RawBytePower    string   `pg:",notnull"`
	QualityAdjPower string   `pg:",notnull"`
}

func (p *PowerActorClaim) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if p == nil {
			return (*PowerActorClaimV0)(nil), true
		}

		return &PowerActorClaimV0{
			Height:          p.Height,
			MinerID:         p.MinerID,
			StateRoot:       p.StateRoot,
			RawBytePower:    p.RawBytePower,
			QualityAdjPower: p.QualityAdjPower,
		}, true
	case 1:
		return p, true
	default:
		return nil, false
	}
}

func (p *PowerActorClaim) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "PowerActorClaim.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "power_actor_claims"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vp, ok := p.AsVersion(version)
	if !ok {
		return xerrors.Errorf("PowerActorClaim not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vp)
}

type PowerActorClaimList []*PowerActorClaim

func (pl PowerActorClaimList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, span := global.Tracer("").Start(ctx, "PowerActorClaimList.Persist")
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "power_actor_claims"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if len(pl) == 0 {
		return nil
	}

	if version.Major != 1 {
		// Support older versions, but in a non-optimal way
		for _, m := range pl {
			if err := m.Persist(ctx, s, version); err != nil {
				return err
			}
		}
		return nil
	}

	return s.PersistModel(ctx, pl)
}

package chain

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

func init() {
	registry.ModelRegistry.Register(registry.ChainEconomicsTask, &ChainEconomics{})
}

type ChainEconomics struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName           struct{} `pg:"chain_economics"`
	Height              int64    `pg:",pk,notnull,use_zero"`
	ParentStateRoot     string   `pg:",notnull"`
	CirculatingFil      string   `pg:"type:numeric,notnull"`
	VestedFil           string   `pg:"type:numeric,notnull"`
	MinedFil            string   `pg:"type:numeric,notnull"`
	BurntFil            string   `pg:"type:numeric,notnull"`
	LockedFil           string   `pg:"type:numeric,notnull"`
	FilReserveDisbursed string   `pg:"type:numeric,notnull"`
}

type ChainEconomicsV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName       struct{} `pg:"chain_economics"`
	ParentStateRoot string   `pg:",notnull"`
	CirculatingFil  string   `pg:",notnull"`
	VestedFil       string   `pg:",notnull"`
	MinedFil        string   `pg:",notnull"`
	BurntFil        string   `pg:",notnull"`
	LockedFil       string   `pg:",notnull"`
}

func (c *ChainEconomics) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if c == nil {
			return (*ChainEconomicsV0)(nil), true
		}

		return &ChainEconomicsV0{
			ParentStateRoot: c.ParentStateRoot,
			CirculatingFil:  c.CirculatingFil,
			VestedFil:       c.VestedFil,
			MinedFil:        c.MinedFil,
			BurntFil:        c.BurntFil,
			LockedFil:       c.LockedFil,
		}, true
	case 1:
		return c, true
	default:
		return nil, false
	}
}

func (c *ChainEconomics) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "chain_economics"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	m, ok := c.AsVersion(version)
	if !ok {
		return xerrors.Errorf("ChainEconomics not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, m)
}

type ChainEconomicsList []*ChainEconomics

func (l ChainEconomicsList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(l) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "ChainEconomicsList.Persist", trace.WithAttributes(label.Int("count", len(l))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "chain_economics"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if version.Major != 1 {
		// Support older versions, but in a non-optimal way
		for _, m := range l {
			if err := m.Persist(ctx, s, version); err != nil {
				return err
			}
		}
		return nil
	}

	return s.PersistModel(ctx, l)
}

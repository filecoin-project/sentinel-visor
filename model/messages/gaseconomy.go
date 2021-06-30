package messages

import (
	"context"

	"github.com/filecoin-project/sentinel-visor/model/registry"
	"go.opencensus.io/tag"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
)

func init() {
	registry.ModelRegistry.Register(registry.MessagesTask, &MessageGasEconomy{})
}

type MessageGasEconomy struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"message_gas_economy"`
	Height    int64    `pg:",pk,notnull,use_zero"`
	StateRoot string   `pg:",pk,notnull"`

	BaseFee          float64 `pg:"type:numeric,use_zero"`
	BaseFeeChangeLog float64 `pg:",use_zero"`

	GasLimitTotal       int64 `pg:"type:numeric,use_zero"`
	GasLimitUniqueTotal int64 `pg:"type:numeric,use_zero"`

	GasFillRatio     float64 `pg:",use_zero"`
	GasCapacityRatio float64 `pg:",use_zero"`
	GasWasteRatio    float64 `pg:",use_zero"`
}

type MessageGasEconomyV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"message_gas_economy"`
	Height    int64    `pg:",pk,notnull,use_zero"`
	StateRoot string   `pg:",pk,notnull"`

	BaseFee          float64 `pg:",use_zero"`
	BaseFeeChangeLog float64 `pg:",use_zero"`

	GasLimitTotal       int64 `pg:",use_zero"`
	GasLimitUniqueTotal int64 `pg:",use_zero"`

	GasFillRatio     float64 `pg:",use_zero"`
	GasCapacityRatio float64 `pg:",use_zero"`
	GasWasteRatio    float64 `pg:",use_zero"`
}

func (g *MessageGasEconomy) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if g == nil {
			return (*MessageGasEconomyV0)(nil), true
		}

		return &MessageGasEconomyV0{
			Height:              g.Height,
			StateRoot:           g.StateRoot,
			BaseFee:             g.BaseFee,
			BaseFeeChangeLog:    g.BaseFeeChangeLog,
			GasLimitTotal:       g.GasLimitTotal,
			GasLimitUniqueTotal: g.GasLimitUniqueTotal,
			GasFillRatio:        g.GasFillRatio,
			GasCapacityRatio:    g.GasCapacityRatio,
			GasWasteRatio:       g.GasWasteRatio,
		}, true
	case 1:
		return g, true
	default:
		return nil, false
	}
}

func (g *MessageGasEconomy) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "message_gas_economy"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vm, ok := g.AsVersion(version)
	if !ok {
		return xerrors.Errorf("MessageGasEconomy not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vm)
}

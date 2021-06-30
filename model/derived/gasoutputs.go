package derived

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
	registry.ModelRegistry.Register(registry.MessagesTask, &GasOutputs{})
}

type GasOutputs struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName          struct{} `pg:"derived_gas_outputs"`
	Height             int64    `pg:",pk,use_zero,notnull"`
	Cid                string   `pg:",pk,notnull"`
	StateRoot          string   `pg:",pk,notnull"`
	From               string   `pg:",notnull"`
	To                 string   `pg:",notnull"`
	Value              string   `pg:"type:numeric,notnull"`
	GasFeeCap          string   `pg:"type:numeric,notnull"`
	GasPremium         string   `pg:"type:numeric,notnull"`
	GasLimit           int64    `pg:",use_zero,notnull"`
	SizeBytes          int      `pg:",use_zero,notnull"`
	Nonce              uint64   `pg:",use_zero,notnull"`
	Method             uint64   `pg:",use_zero,notnull"`
	ActorName          string   `pg:",notnull"`
	ActorFamily        string   `pg:",notnull"`
	ExitCode           int64    `pg:",use_zero,notnull"`
	GasUsed            int64    `pg:",use_zero,notnull"`
	ParentBaseFee      string   `pg:"type:numeric,notnull"`
	BaseFeeBurn        string   `pg:"type:numeric,notnull"`
	OverEstimationBurn string   `pg:"type:numeric,notnull"`
	MinerPenalty       string   `pg:"type:numeric,notnull"`
	MinerTip           string   `pg:"type:numeric,notnull"`
	Refund             string   `pg:"type:numeric,notnull"`
	GasRefund          int64    `pg:",use_zero,notnull"`
	GasBurned          int64    `pg:",use_zero,notnull"`
}

type GasOutputsV0 struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName          struct{} `pg:"derived_gas_outputs"`
	Height             int64    `pg:",pk,use_zero,notnull"`
	Cid                string   `pg:",pk,notnull"`
	StateRoot          string   `pg:",pk,notnull"`
	From               string   `pg:",notnull"`
	To                 string   `pg:",notnull"`
	Value              string   `pg:",notnull"`
	GasFeeCap          string   `pg:",notnull"`
	GasPremium         string   `pg:",notnull"`
	GasLimit           int64    `pg:",use_zero,notnull"`
	SizeBytes          int      `pg:",use_zero,notnull"`
	Nonce              uint64   `pg:",use_zero,notnull"`
	Method             uint64   `pg:",use_zero,notnull"`
	ActorName          string   `pg:",notnull"`
	ExitCode           int64    `pg:",use_zero,notnull"`
	GasUsed            int64    `pg:",use_zero,notnull"`
	ParentBaseFee      string   `pg:",notnull"`
	BaseFeeBurn        string   `pg:",notnull"`
	OverEstimationBurn string   `pg:",notnull"`
	MinerPenalty       string   `pg:",notnull"`
	MinerTip           string   `pg:",notnull"`
	Refund             string   `pg:",notnull"`
	GasRefund          int64    `pg:",use_zero,notnull"`
	GasBurned          int64    `pg:",use_zero,notnull"`
}

func (g *GasOutputs) AsVersion(version model.Version) (interface{}, bool) {
	switch version.Major {
	case 0:
		if g == nil {
			return (*GasOutputsV0)(nil), true
		}

		return &GasOutputsV0{
			Height:             g.Height,
			Cid:                g.Cid,
			StateRoot:          g.StateRoot,
			From:               g.From,
			To:                 g.To,
			Value:              g.Value,
			GasFeeCap:          g.GasFeeCap,
			GasPremium:         g.GasPremium,
			GasLimit:           g.GasLimit,
			SizeBytes:          g.SizeBytes,
			Nonce:              g.Nonce,
			Method:             g.Method,
			ActorName:          g.ActorName,
			ExitCode:           g.ExitCode,
			GasUsed:            g.GasUsed,
			ParentBaseFee:      g.ParentBaseFee,
			BaseFeeBurn:        g.BaseFeeBurn,
			OverEstimationBurn: g.OverEstimationBurn,
			MinerPenalty:       g.MinerPenalty,
			MinerTip:           g.MinerTip,
			Refund:             g.Refund,
			GasRefund:          g.GasRefund,
			GasBurned:          g.GasBurned,
		}, true
	case 1:
		return g, true
	default:
		return nil, false
	}
}

func (g *GasOutputs) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "derived_gas_outputs"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	vg, ok := g.AsVersion(version)
	if !ok {
		return xerrors.Errorf("GasOutputs not supported for schema version %s", version)
	}

	return s.PersistModel(ctx, vg)
}

type GasOutputsList []*GasOutputs

func (l GasOutputsList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(l) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "GasOutputsList.Persist", trace.WithAttributes(label.Int("count", len(l))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "derived_gas_outputs"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	if version.Major != 1 {
		vgl := make([]interface{}, 0, len(l))
		for _, g := range l {
			vg, ok := g.AsVersion(version)
			if !ok {
				return xerrors.Errorf("GasOutputs not supported for schema version %s", version)
			}
			vgl = append(vgl, vg)
		}
		return s.PersistModel(ctx, vgl)
	}

	return s.PersistModel(ctx, l)
}

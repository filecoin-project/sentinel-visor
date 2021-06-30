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
	registry.ModelRegistry.Register(registry.MessagesTask, &InternalMessage{})
}

type InternalMessage struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName     struct{} `pg:"internal_messages"`
	Height        int64    `pg:",pk,notnull,use_zero"`
	Cid           string   `pg:",pk,notnull"`
	StateRoot     string   `pg:",notnull"`
	SourceMessage string
	From          string `pg:",notnull"`
	To            string `pg:",notnull"`
	Value         string `pg:"type:numeric,notnull"`
	Method        uint64 `pg:",use_zero"`
	ActorName     string `pg:",notnull"`
	ActorFamily   string `pg:",notnull"`
	ExitCode      int64  `pg:",use_zero"`
	GasUsed       int64  `pg:",use_zero"`
}

func (im *InternalMessage) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "internal_messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, im)
}

type InternalMessageList []*InternalMessage

func (l InternalMessageList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(l) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "InternalMessageList.Persist", trace.WithAttributes(label.Int("count", len(l))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "internal_messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, l)
}

type InternalParsedMessage struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"internal_parsed_messages"`
	Height    int64    `pg:",pk,notnull,use_zero"`
	Cid       string   `pg:",pk,notnull"`
	From      string   `pg:",notnull"`
	To        string   `pg:",notnull"`
	Value     string   `pg:"type:numeric,notnull"`
	Method    string   `pg:",use_zero"`
	Params    string   `pg:",type:jsonb"`
}

func (ipm *InternalParsedMessage) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "internal_parsed_messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, ipm)
}

type InternalParsedMessageList []*InternalParsedMessage

func (l InternalParsedMessageList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(l) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "InternalParsedMessageList.Persist", trace.WithAttributes(label.Int("count", len(l))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "internal_parsed_messages"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	return s.PersistModel(ctx, l)
}

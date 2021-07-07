package visor

import (
	"context"
	"time"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

type GapReport struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"visor_gap_reports"`

	Height int64  `pg:",pk,use_zero"`
	TipSet string `pg:",pk"`
	Task   string `pg:",pk"`
	Status string `pg:",notnull"`

	// Reporter is the name of the instance that is reporting the result
	Reporter   string    `pg:",notnull"`
	ReportedAt time.Time `pg:",use_zero"`
}

func (p *GapReport) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "visor_gap_reports"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	metrics.RecordCount(ctx, metrics.PersistModel, 1)
	return s.PersistModel(ctx, p)
}

type GapReportList []*GapReport

func (pl GapReportList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(pl) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "GapReportList.Persist", trace.WithAttributes(label.Int("count", len(pl))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "visor_gap_reports"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	metrics.RecordCount(ctx, metrics.PersistModel, len(pl))
	return s.PersistModel(ctx, pl)
}

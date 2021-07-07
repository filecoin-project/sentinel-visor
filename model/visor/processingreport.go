package visor

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

const (
	ProcessingStatusOK    = "OK"
	ProcessingStatusInfo  = "INFO"  // Processing was successful but the task reported information in the StatusInformation column
	ProcessingStatusError = "ERROR" // one or more errors were encountered, data may be incomplete
	ProcessingStatusSkip  = "SKIP"  // no processing was attempted, a reason may be given in the StatusInformation column
)

type ProcessingReport struct {
	//lint:ignore U1000 tableName is a convention used by go-pg
	tableName struct{} `pg:"visor_processing_reports"`

	Height    int64  `pg:",pk,use_zero"`
	StateRoot string `pg:",pk,notnull"`

	// Reporter is the name of the instance that is reporting the result
	Reporter string `pg:",pk,notnull"`

	// Task is the name of the sub task that generated the report
	Task string `pg:",pk,notnull"`

	StartedAt   time.Time `pg:",pk,use_zero"`
	CompletedAt time.Time `pg:",use_zero"`

	Status            string `pg:",notnull"`
	StatusInformation string
	ErrorsDetected    interface{} `pg:",type:jsonb"`
}

func (p *ProcessingReport) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "visor_processing_reports"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	metrics.RecordCount(ctx, metrics.PersistModel, 1)
	return s.PersistModel(ctx, p)
}

type ProcessingReportList []*ProcessingReport

func (pl ProcessingReportList) Persist(ctx context.Context, s model.StorageBatch, version model.Version) error {
	if len(pl) == 0 {
		return nil
	}
	ctx, span := global.Tracer("").Start(ctx, "ProcessingReportList.Persist", trace.WithAttributes(label.Int("count", len(pl))))
	defer span.End()

	ctx, _ = tag.New(ctx, tag.Upsert(metrics.Table, "visor_processing_reports"))
	stop := metrics.Timer(ctx, metrics.PersistDuration)
	defer stop()

	metrics.RecordCount(ctx, metrics.PersistModel, len(pl))
	return s.PersistModel(ctx, pl)
}

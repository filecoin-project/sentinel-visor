package network

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/otel/api/global"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
	"github.com/filecoin-project/sentinel-visor/tasks/observe/peeragents"
	logging "github.com/ipfs/go-log/v2"
)

const (
	PeerAgentsTask = "peeragents" // task that observes connected peer agents
)

var log = logging.Logger("visor/network")

type API interface {
	peeragents.API
}

type ObserverOpt func(t *Observer)

func NewObserver(api API, storage model.Storage, interval time.Duration, name string, tasks []string, options ...ObserverOpt) (*Observer, error) {
	if interval <= 0 {
		return nil, xerrors.Errorf("observer interval must be greater than zero: %d", interval)
	}

	obs := &Observer{
		interval: interval,
		storage:  storage,
		name:     name,
		tasks:    map[string]Task{},
	}

	for _, task := range tasks {
		switch task {
		case PeerAgentsTask:
			obs.tasks[PeerAgentsTask] = peeragents.NewTask(api)
		default:
			return nil, xerrors.Errorf("unknown task: %s", task)
		}
	}

	for _, opt := range options {
		opt(obs)
	}

	return obs, nil
}

// An Observer observes features of the filecoin network
type Observer struct {
	interval time.Duration
	storage  model.Storage
	name     string
	tasks    map[string]Task
}

// Run starts observing the filecoin netwoirk and continues until the context is done or
// a fatal error occurs.
func (o *Observer) Run(ctx context.Context) error {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			o.Tick(ctx)
		}
	}
}

func (o *Observer) Details() (string, map[string]interface{}) {
	return "observer", map[string]interface{}{
		"name":     o.name,
		"interval": o.interval,
	}
}

// Tick is called on each tick of the observer's interval
func (o *Observer) Tick(ctx context.Context) error {
	ctx, span := global.Tracer("").Start(ctx, "Observer.Tick")
	defer span.End()

	// Do as much indexing as possible in the specified time interval
	tctx, cancel := context.WithTimeout(ctx, o.interval)
	defer cancel()

	inFlight := 0
	results := make(chan *TaskResult, len(o.tasks))

	// Run each tipset processing task concurrently
	for name, task := range o.tasks {
		inFlight++
		go o.runTask(tctx, task, name, results)
	}

	// Wait for all tasks to complete
	for inFlight > 0 {
		var res *TaskResult
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res = <-results:
		}
		inFlight--

		llt := log.With("task", res.Task)

		// Was there a fatal error?
		if res.Error != nil {
			llt.Errorw("task returned with error", "error", res.Error.Error())
			return res.Error
		}

		llt.Debugw("task report", "time", res.CompletedAt.Sub(res.StartedAt))

		startPersist := time.Now()
		if err := o.storage.PersistBatch(ctx, res.Data); err != nil {
			stats.Record(ctx, metrics.PersistFailure.M(1))
			llt.Errorw("persistence failed", "error", err)
		} else {
			llt.Debugw("task data persisted", "time", time.Since(startPersist))
		}

	}

	return nil
}

func (o *Observer) runTask(ctx context.Context, task Task, name string, results chan *TaskResult) {
	ctx, _ = tag.New(ctx, tag.Upsert(metrics.TaskType, name))
	stop := metrics.Timer(ctx, metrics.ProcessingDuration)
	defer stop()
	start := time.Now()

	data, err := task.Process(ctx)
	if err != nil {
		stats.Record(ctx, metrics.ProcessingFailure.M(1))
		results <- &TaskResult{
			Task:        name,
			Error:       err,
			StartedAt:   start,
			CompletedAt: time.Now(),
		}
		return
	}
	results <- &TaskResult{
		Task:        name,
		Data:        data,
		StartedAt:   start,
		CompletedAt: time.Now(),
	}
}

// A TaskResult is either some data to persist or an error which indicates that the task did not complete. Partial
// completions are possible provided the Data contains a persistable log of the results.
type TaskResult struct {
	Task        string
	Error       error
	Data        model.Persistable
	StartedAt   time.Time
	CompletedAt time.Time
}

type Task interface {
	Process(ctx context.Context) (model.Persistable, error)
	Close() error
}

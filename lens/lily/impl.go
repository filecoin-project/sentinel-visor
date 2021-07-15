package lily

import (
	"context"
	"sync"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/events"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/impl/common"
	"github.com/filecoin-project/lotus/node/impl/full"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/filecoin-project/sentinel-visor/chain"
	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/lens/util"
	"github.com/filecoin-project/sentinel-visor/network"
	"github.com/filecoin-project/sentinel-visor/schedule"
	"github.com/filecoin-project/sentinel-visor/storage"
)

var _ LilyAPI = (*LilyNodeAPI)(nil)

type LilyNodeAPI struct {
	fx.In

	full.ChainAPI
	full.StateAPI
	full.SyncAPI
	common.CommonAPI
	Events         *events.Events
	Scheduler      *schedule.Scheduler
	StorageCatalog *storage.Catalog
}

func (m *LilyNodeAPI) LilyWatch(_ context.Context, cfg *LilyWatchConfig) (schedule.JobID, error) {
	// the context's passed to these methods live for the duration of the clients request, so make a new one.
	ctx := context.Background()

	// create a database connection for this watch, ensure its pingable, and run migrations if needed/configured to.
	strg, err := m.StorageCatalog.Connect(ctx, cfg.Storage)
	if err != nil {
		return schedule.InvalidJobID, err
	}

	// instantiate an indexer to extract block, message, and actor state data from observed tipsets and persists it to the storage.
	indexer, err := chain.NewTipSetIndexer(m, strg, cfg.Window, cfg.Name, cfg.Tasks)
	if err != nil {
		return schedule.InvalidJobID, err
	}

	// HeadNotifier bridges between the event system and the watcher
	obs := &HeadNotifier{
		bufferSize: 5,
	}

	// get the current head and set it on the tipset cache (mimic chain.watcher behaviour)
	head, err := m.ChainModuleAPI.ChainHead(ctx)
	if err != nil {
		return schedule.InvalidJobID, err
	}

	// Won't block since we are using non-zero buffer size in head notifier
	if err := obs.SetCurrent(ctx, head); err != nil {
		log.Errorw("failed to set current head tipset", "error", err)
	}

	// Hook up the notifier to the event system
	if err := m.Events.Observe(obs); err != nil {
		return schedule.InvalidJobID, err
	}

	id := m.Scheduler.Submit(&schedule.JobConfig{
		Name:                cfg.Name,
		Tasks:               cfg.Tasks,
		Job:                 chain.NewWatcher(indexer, obs, cfg.Confidence),
		RestartOnFailure:    cfg.RestartOnFailure,
		RestartOnCompletion: cfg.RestartOnCompletion,
		RestartDelay:        cfg.RestartDelay,
	})

	return id, nil
}

func (m *LilyNodeAPI) LilyWalk(_ context.Context, cfg *LilyWalkConfig) (schedule.JobID, error) {
	// the context's passed to these methods live for the duration of the clients request, so make a new one.
	ctx := context.Background()

	// create a database connection for this watch, ensure its pingable, and run migrations if needed/configured to.
	strg, err := m.StorageCatalog.Connect(ctx, cfg.Storage)
	if err != nil {
		return schedule.InvalidJobID, err
	}

	// instantiate an indexer to extract block, message, and actor state data from observed tipsets and persists it to the storage.
	indexer, err := chain.NewTipSetIndexer(m, strg, cfg.Window, cfg.Name, cfg.Tasks)
	if err != nil {
		return schedule.InvalidJobID, err
	}

	id := m.Scheduler.Submit(&schedule.JobConfig{
		Name:                cfg.Name,
		Tasks:               cfg.Tasks,
		Job:                 chain.NewWalker(indexer, m, cfg.From, cfg.To),
		RestartOnFailure:    cfg.RestartOnFailure,
		RestartOnCompletion: cfg.RestartOnCompletion,
		RestartDelay:        cfg.RestartDelay,
	})

	return id, nil
}

func (m *LilyNodeAPI) LilyJobStart(_ context.Context, ID schedule.JobID) error {
	if err := m.Scheduler.StartJob(ID); err != nil {
		return err
	}
	return nil
}

func (m *LilyNodeAPI) LilyJobStop(_ context.Context, ID schedule.JobID) error {
	if err := m.Scheduler.StopJob(ID); err != nil {
		return err
	}
	return nil
}

func (m *LilyNodeAPI) LilyJobList(_ context.Context) ([]schedule.JobResult, error) {
	return m.Scheduler.Jobs(), nil
}

func (m *LilyNodeAPI) Open(_ context.Context) (lens.API, lens.APICloser, error) {
	return m, func() {}, nil
}

func (m *LilyNodeAPI) GetExecutedAndBlockMessagesForTipset(ctx context.Context, ts, pts *types.TipSet) (*lens.TipSetMessages, error) {
	return util.GetExecutedAndBlockMessagesForTipset(ctx, m.ChainAPI.Chain, ts, pts)
}

func (m *LilyNodeAPI) Store() adt.Store {
	return m.ChainAPI.Chain.ActorStore(context.TODO())
}

func (m *LilyNodeAPI) StateGetReceipt(ctx context.Context, msg cid.Cid, from types.TipSetKey) (*types.MessageReceipt, error) {
	ml, err := m.StateSearchMsg(ctx, from, msg, api.LookbackNoLimit, true)
	if err != nil {
		return nil, err
	}

	if ml == nil {
		return nil, nil
	}

	return &ml.Receipt, nil
}

func (m *LilyNodeAPI) LogList(ctx context.Context) ([]string, error) {
	return logging.GetSubsystems(), nil
}

func (m *LilyNodeAPI) LogSetLevel(ctx context.Context, subsystem, level string) error {
	return logging.SetLogLevel(subsystem, level)
}

func (m *LilyNodeAPI) Shutdown(ctx context.Context) error {
	m.ShutdownChan <- struct{}{}
	return nil
}

func (m *LilyNodeAPI) LilyObserve(_ context.Context, cfg *LilyObserveConfig) (schedule.JobID, error) {
	// the context's passed to these methods live for the duration of the clients request, so make a new one.
	ctx := context.Background()

	// create a database connection for this watch, ensure its pingable, and run migrations if needed/configured to.
	strg, err := m.StorageCatalog.Connect(ctx, cfg.Storage)
	if err != nil {
		return schedule.InvalidJobID, err
	}

	// instantiate an indexer to extract block, message, and actor state data from observed tipsets and persists it to the storage.
	obs, err := network.NewObserver(m, strg, cfg.Interval, cfg.Name, cfg.Tasks)
	if err != nil {
		return schedule.InvalidJobID, err
	}

	id := m.Scheduler.Submit(&schedule.JobConfig{
		Name:                cfg.Name,
		Tasks:               cfg.Tasks,
		Job:                 obs,
		RestartOnFailure:    cfg.RestartOnFailure,
		RestartOnCompletion: cfg.RestartOnCompletion,
		RestartDelay:        cfg.RestartDelay,
	})

	return id, nil
}

type HeadNotifier struct {
	mu     sync.Mutex            // protects following fields
	events chan *chain.HeadEvent // created lazily, closed by first cancel call
	err    error                 // set to non-nil by the first cancel call

	// size of the buffer to maintain for events. Using a buffer reduces chance
	// that the emitter of events will block when sending to this notifier.
	bufferSize int
}

func (h *HeadNotifier) eventsCh() chan *chain.HeadEvent {
	// caller must hold mu
	if h.events == nil {
		h.events = make(chan *chain.HeadEvent, h.bufferSize)
	}
	return h.events
}

func (h *HeadNotifier) HeadEvents() <-chan *chain.HeadEvent {
	h.mu.Lock()
	ev := h.eventsCh()
	h.mu.Unlock()
	return ev
}

func (h *HeadNotifier) Err() error {
	h.mu.Lock()
	err := h.err
	h.mu.Unlock()
	return err
}

func (h *HeadNotifier) Cancel(err error) {
	h.mu.Lock()
	if h.err != nil {
		h.mu.Unlock()
		return
	}
	h.err = err
	if h.events == nil {
		h.events = make(chan *chain.HeadEvent, h.bufferSize)
	}
	close(h.events)
	h.mu.Unlock()
}

func (h *HeadNotifier) SetCurrent(ctx context.Context, ts *types.TipSet) error {
	h.mu.Lock()
	if h.err != nil {
		err := h.err
		h.mu.Unlock()
		return err
	}
	ev := h.eventsCh()
	h.mu.Unlock()

	// This is imprecise since it's inherently racy but good enough to emit
	// a warning that the event may block the sender
	if len(ev) == cap(ev) {
		log.Warnw("head notifier buffer at capacity", "queued", len(ev))
	}

	ev <- &chain.HeadEvent{
		Type:   chain.HeadEventCurrent,
		TipSet: ts,
	}
	return nil
}

func (h *HeadNotifier) Apply(ctx context.Context, ts *types.TipSet) error {
	h.mu.Lock()
	if h.err != nil {
		err := h.err
		h.mu.Unlock()
		return err
	}
	ev := h.eventsCh()
	h.mu.Unlock()

	// This is imprecise since it's inherently racy but good enough to emit
	// a warning that the event may block the sender
	if len(ev) == cap(ev) {
		log.Warnw("head notifier buffer at capacity", "queued", len(ev))
	}

	ev <- &chain.HeadEvent{
		Type:   chain.HeadEventApply,
		TipSet: ts,
	}
	return nil
}

func (h *HeadNotifier) Revert(ctx context.Context, ts *types.TipSet) error {
	h.mu.Lock()
	if h.err != nil {
		err := h.err
		h.mu.Unlock()
		return err
	}
	ev := h.eventsCh()
	h.mu.Unlock()

	// This is imprecise since it's inherently racy but good enough to emit
	// a warning that the event may block the sender
	if len(ev) == cap(ev) {
		log.Warnw("head notifier buffer at capacity", "queued", len(ev))
	}

	ev <- &chain.HeadEvent{
		Type:   chain.HeadEventRevert,
		TipSet: ts,
	}
	return nil
}

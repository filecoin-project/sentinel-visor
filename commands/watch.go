package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	store "github.com/filecoin-project/lotus/chain/store"
	lotuscli "github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/sentinel-visor/schedule"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/urfave/cli/v2"
	"go.opencensus.io/stats"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/chain"
	"github.com/filecoin-project/sentinel-visor/lens"
	"github.com/filecoin-project/sentinel-visor/lens/lily"
	"github.com/filecoin-project/sentinel-visor/metrics"
	"github.com/filecoin-project/sentinel-visor/model"
	"github.com/filecoin-project/sentinel-visor/storage"
)

type watchOps struct {
	confidence int
	tasks      string
	window     time.Duration
	storage    string
	apiAddr    string
	apiToken   string
	name       string
}

var watchFlags watchOps

var WatchCmd = &cli.Command{
	Name:  "watch",
	Usage: "Start a daemon job to watch the head of the filecoin blockchain.",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:        "confidence",
			Usage:       "Sets the size of the cache used to hold tipsets for possible reversion before being committed to the database",
			Value:       2,
			Destination: &watchFlags.confidence,
		},
		&cli.StringFlag{
			Name:        "tasks",
			Usage:       "Comma separated list of tasks to run. Each task is reported separately in the database.",
			Value:       strings.Join([]string{chain.BlocksTask, chain.MessagesTask, chain.ChainEconomicsTask, chain.ActorStatesRawTask}, ","),
			Destination: &watchFlags.tasks,
		},
		&cli.DurationFlag{
			Name:        "window",
			Usage:       "Duration after which any indexing work not completed will be marked incomplete",
			Value:       builtin.EpochDurationSeconds * time.Second,
			Destination: &watchFlags.window,
		},
		&cli.StringFlag{
			Name:        "storage",
			Usage:       "Name of storage that results will be written to.",
			Value:       "",
			Destination: &watchFlags.storage,
		},
		&cli.StringFlag{
			Name:        "api",
			Usage:       "Address of visor api in multiaddr format.",
			EnvVars:     []string{"VISOR_API"},
			Value:       "/ip4/127.0.0.1/tcp/1234",
			Destination: &watchFlags.apiAddr,
		},
		&cli.StringFlag{
			Name:        "api-token",
			Usage:       "Authentication token for visor api.",
			EnvVars:     []string{"VISOR_API_TOKEN"},
			Value:       "",
			Destination: &watchFlags.apiToken,
		},
		&cli.StringFlag{
			Name:        "name",
			Usage:       "Name of job for easy identification later.",
			Value:       "",
			Destination: &watchFlags.name,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := lotuscli.ReqContext(cctx)

		watchName := fmt.Sprintf("watch_%d", time.Now().Unix())
		if watchFlags.name != "" {
			watchName = watchFlags.name
		}

		cfg := &lily.LilyWatchConfig{
			Name:                watchName,
			Tasks:               strings.Split(watchFlags.tasks, ","),
			Window:              watchFlags.window,
			Confidence:          watchFlags.confidence,
			RestartDelay:        0,
			RestartOnCompletion: false,
			RestartOnFailure:    true,
			Storage:             watchFlags.storage,
		}

		api, closer, err := GetAPI(ctx, watchFlags.apiAddr, watchFlags.apiToken)
		if err != nil {
			return err
		}
		defer closer()

		watchID, err := api.LilyWatch(ctx, cfg)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "Created Watch Job: %d\n", watchID); err != nil {
			return err
		}
		return nil
	},
}

var RunWatchCmd = &cli.Command{
	Name:  "watch",
	Usage: "Watch the head of the filecoin blockchain and process blocks as they arrive.",
	Flags: flagSet(
		dbConnectFlags,
		dbBehaviourFlags,
		runLensFlags,
		[]cli.Flag{
			&cli.IntFlag{
				Name:    "indexhead-confidence",
				Usage:   "Sets the size of the cache used to hold tipsets for possible reversion before being committed to the database",
				Value:   2,
				EnvVars: []string{"VISOR_INDEXHEAD_CONFIDENCE"},
			},
			&cli.StringFlag{
				Name:    "tasks",
				Usage:   "Comma separated list of tasks to run. Each task is reported separately in the database.",
				Value:   strings.Join([]string{chain.BlocksTask, chain.MessagesTask, chain.ChainEconomicsTask, chain.ActorStatesRawTask}, ","),
				EnvVars: []string{"VISOR_WATCH_TASKS"},
			},
			&cli.DurationFlag{
				Name:   "window",
				Usage:  "Time window in which data extraction must be completed.",
				Value:  builtin.EpochDurationSeconds * time.Second,
				Hidden: true,
			},
		},
	),
	Action: runWatch,
}

func runWatch(cctx *cli.Context) error {
	tasks := strings.Split(cctx.String("tasks"), ",")

	if err := setupLogging(cctx); err != nil {
		return xerrors.Errorf("setup logging: %w", err)
	}

	if err := setupMetrics(cctx); err != nil {
		return xerrors.Errorf("setup metrics: %w", err)
	}

	tcloser, err := setupTracing(cctx)
	if err != nil {
		return xerrors.Errorf("setup tracing: %w", err)
	}
	defer tcloser()

	lensOpener, lensCloser, err := setupLens(cctx)
	if err != nil {
		return xerrors.Errorf("setup lens: %w", err)
	}
	defer func() {
		lensCloser()
	}()

	var storage model.Storage = &storage.NullStorage{}
	if cctx.String("db") == "" {
		log.Warnw("database not specified, data will not be persisted")
	} else {
		db, err := setupDatabase(cctx)
		if err != nil {
			return xerrors.Errorf("setup database: %w", err)
		}
		storage = db
	}

	tsIndexer, err := chain.NewTipSetIndexer(lensOpener, storage, cctx.Duration("window"), cctx.String("name"), tasks)
	if err != nil {
		return xerrors.Errorf("setup indexer: %w", err)
	}

	notifier := NewLotusChainNotifier(lensOpener)

	// TODO scheduler does not respect the ordering of these jobs, make it respect jobID when starting.
	// Subscribe to chain head events to be passed to the watcher
	scheduler := schedule.NewScheduler(cctx.Duration("task-delay"), &schedule.JobConfig{
		Name:                "ChainHeadNotifier",
		Job:                 notifier,
		RestartOnFailure:    true,
		RestartOnCompletion: true, // we always want the notifier to be running
		RestartDelay:        time.Minute,
	}, &schedule.JobConfig{
		Name: "Watcher",
		Job:  chain.NewWatcher(tsIndexer, notifier, cctx.Int("indexhead-confidence")),
		// TODO: add locker
		// Locker:              NewGlobalSingleton(ChainHeadIndexerLockID, rctx.db), // only want one forward indexer anywhere to be running
		RestartOnFailure:    true,
		RestartOnCompletion: true, // we always want the indexer to be running
		RestartDelay:        time.Minute,
	})

	// Start the scheduler and wait for it to complete or to be cancelled.
	err = scheduler.Run(cctx.Context)
	if !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// LotusChainNotifier is a head event notifier that subscribes to a lens's ChainNotify method and adapts the
// events received for use by a chain.Watcher
// NOTE: this functionality will be probably folded into the Lotus API lens since other lenses will support more
// direct methods of accessing new tipsets
type LotusChainNotifier struct {
	opener lens.APIOpener

	mu     sync.Mutex            // protects following fields
	events chan *chain.HeadEvent // initialised in NewLotusChainNotifier and never mutated but may be closed
	err    error                 // set to non-nil by the first cancel call. If non-nil then events channel has been closed.
}

func NewLotusChainNotifier(opener lens.APIOpener) *LotusChainNotifier {
	return &LotusChainNotifier{
		opener: opener,
		events: make(chan *chain.HeadEvent),
	}
}

func (c *LotusChainNotifier) HeadEvents() <-chan *chain.HeadEvent {
	return c.events
}

func (c *LotusChainNotifier) Err() error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	return err
}

func (c *LotusChainNotifier) Cancel(err error) {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	if err != nil {
		c.err = err
	} else {
		c.err = fmt.Errorf("canceled")
	}
	close(c.events)
	c.mu.Unlock()
}

// Run subscribes to ChainNotify and blocks until the context is done or
// an error occurs.
func (c *LotusChainNotifier) Run(ctx context.Context) error {
	node, closer, err := c.opener.Open(ctx)
	if err != nil {
		return xerrors.Errorf("open lens: %w", err)
	}
	defer closer()
	log.Debugw("lens opened")

	hc, err := node.ChainNotify(ctx)
	if err != nil {
		return xerrors.Errorf("chain notify: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case headEvents, ok := <-hc:
			if !ok {
				return xerrors.Errorf("ChainNotify channel closed")
			}

			for _, ch := range headEvents {
				stats.Record(ctx, metrics.WatchHeight.M(int64(ch.Val.Height())))
				he := &chain.HeadEvent{
					TipSet: ch.Val,
				}
				switch ch.Type {
				case store.HCCurrent:
					he.Type = chain.HeadEventCurrent
				case store.HCApply:
					he.Type = chain.HeadEventApply
				case store.HCRevert:
					he.Type = chain.HeadEventRevert
				}

				// Must take the lock here to test if events channel has been closed by a call to cancel
				c.mu.Lock()
				if c.err == nil {
					c.events <- he
				}
				c.mu.Unlock()
			}
		}
	}
}

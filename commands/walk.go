package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	lotuscli "github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/sentinel-visor/chain/actors/builtin"
	"github.com/filecoin-project/sentinel-visor/lens/lily"
	"github.com/filecoin-project/sentinel-visor/model/registry"
	"github.com/filecoin-project/sentinel-visor/schedule"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/chain"
	"github.com/filecoin-project/sentinel-visor/model"
	"github.com/filecoin-project/sentinel-visor/storage"
)

type walkOps struct {
	from     int64
	to       int64
	tasks    string
	window   time.Duration
	storage  string
	apiAddr  string
	apiToken string
	name     string
}

var walkFlags walkOps

var WalkCmd = &cli.Command{
	Name:  "walk",
	Usage: "Start a daemon job to walk a range of the filecoin blockchain.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "tasks",
			Usage:       "Comma separated list of tasks to run. Each task is reported separately in the database.",
			Value:       strings.Join([]string{registry.BlocksTask, registry.MessagesTask, registry.ChainEconomicsTask, registry.ActorStatesRawTask}, ","),
			Destination: &walkFlags.tasks,
		},
		&cli.DurationFlag{
			Name:        "window",
			Usage:       "Duration after which any indexing work not completed will be marked incomplete",
			Value:       builtin.EpochDurationSeconds * time.Second * 10, // walks don't need to complete within a single epoch
			Destination: &walkFlags.window,
		},
		&cli.Int64Flag{
			Name:        "from",
			Usage:       "Limit actor and message processing to tipsets at or above `HEIGHT`",
			Destination: &walkFlags.from,
		},
		&cli.Int64Flag{
			Name:        "to",
			Usage:       "Limit actor and message processing to tipsets at or below `HEIGHT`",
			Value:       estimateCurrentEpoch(),
			DefaultText: "MaxInt64",
			Destination: &walkFlags.to,
		},
		&cli.StringFlag{
			Name:        "storage",
			Usage:       "Name of storage that results will be written to.",
			Value:       "",
			Destination: &walkFlags.storage,
		},
		&cli.StringFlag{
			Name:        "api",
			Usage:       "Address of visor api in multiaddr format.",
			EnvVars:     []string{"VISOR_API"},
			Value:       "/ip4/127.0.0.1/tcp/1234",
			Destination: &walkFlags.apiAddr,
		},
		&cli.StringFlag{
			Name:        "api-token",
			Usage:       "Authentication token for visor api.",
			EnvVars:     []string{"VISOR_API_TOKEN"},
			Value:       "",
			Destination: &walkFlags.apiToken,
		},
		&cli.StringFlag{
			Name:        "name",
			Usage:       "Name of job for easy identification later.",
			Value:       "",
			Destination: &walkFlags.name,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := lotuscli.ReqContext(cctx)

		walkName := fmt.Sprintf("walk_%d", time.Now().Unix())
		if walkFlags.name != "" {
			walkName = walkFlags.name
		}

		cfg := &lily.LilyWalkConfig{
			Name:                walkName,
			Tasks:               strings.Split(walkFlags.tasks, ","),
			Window:              walkFlags.window,
			From:                walkFlags.from,
			To:                  walkFlags.to,
			RestartDelay:        0,
			RestartOnCompletion: false,
			RestartOnFailure:    false,
			Storage:             walkFlags.storage,
		}

		api, closer, err := GetAPI(ctx, walkFlags.apiAddr, walkFlags.apiToken)
		if err != nil {
			return err
		}
		defer closer()

		watchID, err := api.LilyWalk(ctx, cfg)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "Created Watch Job: %d", watchID); err != nil {
			return err
		}
		return nil
	},
}

var RunWalkCmd = &cli.Command{
	Name:  "walk",
	Usage: "Walk a range of the filecoin blockchain and process blocks as they are discovered.",
	Flags: flagSet(
		dbConnectFlags,
		dbBehaviourFlags,
		runLensFlags,
		[]cli.Flag{
			&cli.Int64Flag{
				Name:    "from",
				Usage:   "Limit actor and message processing to tipsets at or above `HEIGHT`",
				EnvVars: []string{"VISOR_HEIGHT_FROM"},
			},
			&cli.Int64Flag{
				Name:        "to",
				Usage:       "Limit actor and message processing to tipsets at or below `HEIGHT`",
				Value:       estimateCurrentEpoch(),
				DefaultText: "MaxInt64",
				EnvVars:     []string{"VISOR_HEIGHT_TO"},
			},
			&cli.StringFlag{
				Name:    "tasks",
				Usage:   "Comma separated list of tasks to run. Each task is reported separately in the database.",
				Value:   strings.Join([]string{registry.BlocksTask, registry.MessagesTask, registry.ChainEconomicsTask, registry.ActorStatesRawTask}, ","),
				EnvVars: []string{"VISOR_WALK_TASKS"},
			},
			&cli.StringFlag{
				Name:   "csv",
				Usage:  "Path to write csv files.",
				Hidden: true,
			},
		},
	),
	Action: func(cctx *cli.Context) error {
		// Validate flags
		heightFrom := cctx.Int64("from")
		heightTo := cctx.Int64("to")

		if heightFrom > heightTo {
			return xerrors.Errorf("--from must not be greater than --to")
		}

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

		var strg model.Storage = &storage.NullStorage{}
		if cctx.String("csv") != "" {
			csvStorage, err := storage.NewCSVStorageLatest(cctx.String("csv"))
			if err != nil {
				return xerrors.Errorf("new csv storage: %w", err)
			}
			strg = csvStorage
		} else {
			if cctx.String("db") == "" {
				log.Warnw("database not specified, data will not be persisted")
			} else {
				db, err := setupDatabase(cctx)
				if err != nil {
					return xerrors.Errorf("setup database: %w", err)
				}
				strg = db
			}
		}

		tsIndexer, err := chain.NewTipSetIndexer(lensOpener, strg, 0, cctx.String("name"), tasks)
		if err != nil {
			return xerrors.Errorf("setup indexer: %w", err)
		}

		scheduler := schedule.NewScheduler(cctx.Duration("task-delay"),
			&schedule.JobConfig{
				Name:                "Walker",
				Job:                 chain.NewWalker(tsIndexer, lensOpener, heightFrom, heightTo),
				RestartOnFailure:    false, // Don't restart after a failure otherwise the walk will start from the beginning again
				RestartOnCompletion: false,
				RestartDelay:        time.Minute,
			})

		err = scheduler.Run(cctx.Context)
		if !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	},
}

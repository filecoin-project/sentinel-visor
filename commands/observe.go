package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	lotuscli "github.com/filecoin-project/lotus/cli"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/sentinel-visor/lens/lily"
	"github.com/filecoin-project/sentinel-visor/network"
)

var observeFlags struct {
	tasks    string
	storage  string
	name     string
	interval time.Duration
}

var ObserveCmd = &cli.Command{
	Name:  "observe",
	Usage: "Start a daemon job to observe the node and its environment.",
	Flags: flagSet(
		clientAPIFlagSet,
		[]cli.Flag{
			&cli.StringFlag{
				Name:        "tasks",
				Usage:       "Comma separated list of observe tasks to run. Each task is reported separately in the database.",
				Value:       strings.Join([]string{network.PeerAgentsTask}, ","),
				Destination: &observeFlags.tasks,
			},
			&cli.DurationFlag{
				Name:        "interval",
				Usage:       "Duration after which any indexing work not completed will be marked incomplete",
				Value:       600 * time.Second,
				Destination: &observeFlags.interval,
			},
			&cli.StringFlag{
				Name:        "storage",
				Usage:       "Name of storage that results will be written to.",
				Value:       "",
				Destination: &observeFlags.storage,
			},
			&cli.StringFlag{
				Name:        "name",
				Usage:       "Name of job for easy identification later.",
				Value:       "",
				Destination: &observeFlags.name,
			},
		},
	),
	Action: func(cctx *cli.Context) error {
		ctx := lotuscli.ReqContext(cctx)

		obsName := fmt.Sprintf("observe_%d", time.Now().Unix())
		if observeFlags.name != "" {
			obsName = observeFlags.name
		}

		cfg := &lily.LilyObserveConfig{
			Name:                obsName,
			Tasks:               strings.Split(observeFlags.tasks, ","),
			Interval:            observeFlags.interval,
			RestartDelay:        0,
			RestartOnCompletion: false,
			RestartOnFailure:    true,
			Storage:             observeFlags.storage,
		}

		api, closer, err := GetAPI(ctx, clientAPIFlags.apiAddr, clientAPIFlags.apiToken)
		if err != nil {
			return err
		}
		defer closer()

		obsID, err := api.LilyObserve(ctx, cfg)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "Created observe job %s (%d)\n", obsName, obsID); err != nil {
			return err
		}
		return nil
	},
}

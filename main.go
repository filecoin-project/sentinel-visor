package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/stmgr"
	"github.com/filecoin-project/lotus/node/repo"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/sentinel-visor/commands"
	"github.com/filecoin-project/sentinel-visor/version"
)

var log = logging.Logger("visor/main")

type UpSchedule struct {
	Height    int64
	Network   uint
	Expensive bool
}

func (u *UpSchedule) String() string {
	return fmt.Sprintf("Height: %d, Network: %d, Expensive: %t", u.Height, u.Network, u.Expensive)
}

type UpScheduleList []*UpSchedule

func (ul UpScheduleList) String() string {
	var sb strings.Builder
	for _, u := range ul {
		sb.WriteString(fmt.Sprintln("\t\t" + u.String()))
	}
	return sb.String()
}

func main() {
	// Set up a context that is canceled when the command is interrupted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a signal handler to cancel the context
	go func() {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT)
		select {
		case <-interrupt:
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := logging.SetLogLevel("*", "info"); err != nil {
		log.Fatal(err)
	}

	var up UpScheduleList
	for _, u := range stmgr.DefaultUpgradeSchedule() {
		up = append(up, &UpSchedule{
			Height:    int64(u.Height),
			Network:   uint(u.Network),
			Expensive: false,
		})
	}

	app := &cli.App{
		Name:    "visor",
		Usage:   "Filecoin Chain Monitoring Utility",
		Version: fmt.Sprintf("VisorVersion: \t%s\n   NewestNetworkVersion: \t%d\n   GenesisFile: \t%s\n   DevNet: \t%t\n   UserVersion: \t%s\n   UpgradeSchedule: \n%s", version.String(), build.NewestNetworkVersion, build.GenesisFile, build.Devnet, build.UserVersion(), up.String()),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				EnvVars:     []string{"GOLOG_LOG_LEVEL"},
				Value:       "debug",
				Usage:       "Set the default log level for all loggers to `LEVEL`",
				Destination: &commands.VisorCmdFlags.LogLevel,
			},
			&cli.StringFlag{
				Name:        "log-level-named",
				EnvVars:     []string{"VISOR_LOG_LEVEL_NAMED"},
				Value:       "",
				Usage:       "A comma delimited list of named loggers and log levels formatted as name:level, for example 'logger1:debug,logger2:info'",
				Destination: &commands.VisorCmdFlags.LogLevelNamed,
			},
			&cli.BoolFlag{
				Name:        "tracing",
				EnvVars:     []string{"VISOR_TRACING"},
				Value:       false,
				Destination: &commands.VisorCmdFlags.Tracing,
			},
			&cli.StringFlag{
				Name:        "jaeger-agent-host",
				EnvVars:     []string{"JAEGER_AGENT_HOST"},
				Value:       "localhost",
				Destination: &commands.VisorCmdFlags.JaegerHost,
			},
			&cli.IntFlag{
				Name:        "jaeger-agent-port",
				EnvVars:     []string{"JAEGER_AGENT_PORT"},
				Value:       6831,
				Destination: &commands.VisorCmdFlags.JaegerPort,
			},
			&cli.StringFlag{
				Name:        "jaeger-service-name",
				EnvVars:     []string{"JAEGER_SERVICE_NAME"},
				Value:       "visor",
				Destination: &commands.VisorCmdFlags.JaegerName,
			},
			&cli.StringFlag{
				Name:        "jaeger-sampler-type",
				EnvVars:     []string{"JAEGER_SAMPLER_TYPE"},
				Value:       "probabilistic",
				Destination: &commands.VisorCmdFlags.JaegerSampleType,
			},
			&cli.Float64Flag{
				Name:        "jaeger-sampler-param",
				EnvVars:     []string{"JAEGER_SAMPLER_PARAM"},
				Value:       0.0001,
				Destination: &commands.VisorCmdFlags.JaegerSamplerParam,
			},
			&cli.StringFlag{
				Name:        "prometheus-port",
				EnvVars:     []string{"VISOR_PROMETHEUS_PORT"},
				Value:       ":9991",
				Destination: &commands.VisorCmdFlags.PrometheusPort,
			},
		},
		Commands: []*cli.Command{
			commands.DaemonCmd,
			commands.GapCmd,
			commands.InitCmd,
			commands.JobCmd,
			commands.LogCmd,
			commands.MigrateCmd,
			commands.NetCmd,
			commands.RunCmd,
			commands.StopCmd,
			commands.SyncCmd,
			commands.VectorCmd,
			commands.WaitApiCmd,
			commands.WatchCmd,
			commands.WalkCmd,
		},
	}
	app.Setup()
	app.Metadata["repoType"] = repo.FullNode
	app.Metadata["traceContext"] = ctx

	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

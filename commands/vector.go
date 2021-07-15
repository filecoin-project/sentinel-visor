package commands

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sentinel-visor/chain"
	"github.com/filecoin-project/sentinel-visor/vector"
)

var VectorCmd = &cli.Command{
	Name:  "vector",
	Usage: "Vector tooling for Visor.",
	Subcommands: []*cli.Command{
		BuildVectorCmd,
		ExecuteVectorCmd,
	},
}

var BuildVectorCmd = &cli.Command{
	Name:  "build",
	Usage: "Create a vector.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lens-repo",
			EnvVars: []string{"VISOR_LENS_REPO"},
			Value:   "~/.lotus", // TODO: Consider XDG_DATA_HOME
			Usage:   "The path of a repo to be opened by the lens",
		},
		&cli.Int64Flag{
			Name:    "from",
			Usage:   "Limit actor and message processing to tipsets at or above `HEIGHT`",
			EnvVars: []string{"VISOR_HEIGHT_FROM"},
		},
		&cli.Int64Flag{
			Name:        "to",
			Usage:       "Limit actor and message processing to tipsets at or below `HEIGHT`",
			Value:       estimateCurrentEpoch(),
			DefaultText: "current epoch",
			EnvVars:     []string{"VISOR_HEIGHT_TO"},
		},
		&cli.StringFlag{
			Name:    "tasks",
			Usage:   "Comma separated list of tasks to build. Each task is reported separately in the database.",
			Value:   strings.Join([]string{chain.BlocksTask}, ","),
			EnvVars: []string{"VISOR_VECTOR_TASKS"},
		},
		&cli.StringFlag{
			Name:  "actor-address",
			Usage: "Address of an actor.",
		},
		&cli.StringFlag{
			Name:     "vector-file",
			Usage:    "Path of vector file.",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "vector-desc",
			Usage:    "Short description of the test vector.",
			Required: true,
		},
	},
	Action: build,
}

func build(cctx *cli.Context) error {
	// Set up a context that is canceled when the command is interrupted
	ctx, cancel := context.WithCancel(cctx.Context)

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

	if err := setupLogging(cctx); err != nil {
		return xerrors.Errorf("setup logging: %w", err)
	}

	builder, err := vector.NewBuilder(cctx)
	if err != nil {
		return err
	}

	schema, err := builder.Build(ctx)
	if err != nil {
		return err
	}

	return schema.Persist(cctx.String("vector-file"))
}

var ExecuteVectorCmd = &cli.Command{
	Name:  "execute",
	Usage: "execute a test vector",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "vector-file",
			Usage:    "Path to vector file.",
			Required: true,
		},
	},
	Action: execute,
}

func execute(cctx *cli.Context) error {
	// Set up a context that is canceled when the command is interrupted
	ctx, cancel := context.WithCancel(cctx.Context)

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

	if err := setupLogging(cctx); err != nil {
		return xerrors.Errorf("setup logging: %w", err)
	}
	runner, err := vector.NewRunner(ctx, cctx.String("vector-file"), 0)
	if err != nil {
		return err
	}

	err = runner.Run(ctx)
	if err != nil {
		return err
	}

	return runner.Validate(ctx)
}

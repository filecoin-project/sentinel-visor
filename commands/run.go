package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/sentinel-visor/version"
)

var defaultName = "visor"

func init() {
	defaultName = "visor_" + version.String()
	hostname, err := os.Hostname()
	if err == nil {
		defaultName = fmt.Sprintf("%s_%s_%d", defaultName, hostname, os.Getpid())
	}
}

var RunCmd = &cli.Command{
	Name:  "run",
	Usage: "Run a single job without starting a daemon.",
	Description: `Starts visor in a standalone mode to run a single job using an external source
of block data. Visor exits when the job is complete or when a fatal error
occurs.

To start a watch use 'visor run watch'. A watch does not terminate but follows
the chain forever.

To start a walk use 'visor run walk' and specify the range of heights
using --from and --to.  A walk job completes when it has traversed all tipsets
in the requested range including the start and end heights.

The source of block data is specified by passing the --lens option to the 'walk'
or 'watch' command. Several lenses are supported:

  lotus       Obtain block data using the JSON-RPC API of a Lotus node.
              Use the --lens-lotus-api option to specify the address if the
              Lotus node.

  lotusrepo   Read block data from a Lotus node's repository on disk.
              Use the --lens-repo option to specify the directory containing
              the repository.

  carrepo     Read block data from a block archive in car format.
              Use the --lens-repo option to specify the name of the car file.

Use the --csv option to specify a directory to write the extracted data in CSV
format.

Use the --db option to specify a connection string for the TimescaleDB
database. See 'visor help schema' for more information on the database schema
used.

Note that the volume of data and number of API requests needed to fully extract
actor states can be extremely high. Running actor tasks against a running Lotus
node is not recommended and may cause the node to fall out of sync. Consider
operating visor in daemon mode instead (see 'visor help daemon') or, for
historic walks, use a copy of the lotus node's blockstore with the 'lotusrepo'
lens.
`,

	Subcommands: []*cli.Command{
		RunWatchCmd,
		RunWalkCmd,
	},
}

var dbConnectFlags = []cli.Flag{
	&cli.StringFlag{
		Name:    "db",
		EnvVars: []string{"LOTUS_DB"},
		Value:   "",
		Usage:   "A connection string for the TimescaleDB database, for example postgres://postgres:password@localhost:5432/postgres",
	},
	&cli.IntFlag{
		Name:    "db-pool-size",
		EnvVars: []string{"LOTUS_DB_POOL_SIZE"},
		Value:   75,
	},
	&cli.StringFlag{
		Name:    "name",
		EnvVars: []string{"VISOR_NAME"},
		Value:   defaultName,
		Usage:   "A name that helps to identify this instance of visor.",
	},
	&cli.StringFlag{
		Name:    "schema",
		EnvVars: []string{"VISOR_SCHEMA"},
		Value:   "public",
		Usage:   "The name of the postgresql schema that holds the objects used by this instance of visor.",
	},
}

var dbBehaviourFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:    "db-allow-upsert",
		EnvVars: []string{"LOTUS_DB_ALLOW_UPSERT"},
		Value:   false,
	},
	&cli.BoolFlag{
		Name:    "allow-schema-migration",
		EnvVars: []string{"VISOR_ALLOW_SCHEMA_MIGRATION"},
		Value:   false,
	},
}

var runLensFlags = []cli.Flag{
	&cli.StringFlag{
		Name:    "lens",
		EnvVars: []string{"VISOR_LENS"},
		Value:   "lotus",
	},
	&cli.StringFlag{
		Name:    "lens-lotus-api",
		EnvVars: []string{"VISOR_LENS_LOTUS_API"},
		Value:   "",
		Usage:   "The multiaddress of a lotus API, needed when using the lotus lens",
	},
	&cli.StringFlag{
		Name:    "lens-repo",
		EnvVars: []string{"VISOR_LENS_REPO"},
		Value:   "~/.lotus", // TODO: Consider XDG_DATA_HOME
		Usage:   "The path of a repo to be opened by the lens",
	},
	&cli.IntFlag{
		Name:    "lens-cache-hint",
		EnvVars: []string{"VISOR_LENS_CACHE_HINT"},
		Value:   1024 * 1024,
	},
	&cli.StringFlag{
		Name:    "lens-postgres-namespace",
		EnvVars: []string{"VISOR_LENS_POSTGRES_NAMESPACE"},
		Value:   "main", // we need *some* namespace specified, otherwise GetFilTipSetHead() can't work
		Usage:   "Namespace consulted for current chain head and recency records",
	},
	&cli.BoolFlag{
		Name:    "lens-postgres-preload-recents",
		EnvVars: []string{"VISOR_LENS_POSTGRES_PRELOAD_RECENTS"},
		Value:   false,
		Usage:   "List recent reads within selected namespace, and preload as much as possible into the LRU",
	},
	&cli.IntFlag{
		Name:    "lens-postgres-get-prefetch-depth",
		EnvVars: []string{"VISOR_LENS_POSTGRES_GET_PREFETCH_DEPTH"},
		Value:   0,
		Usage:   "Prefetch that many additional DAG layers of descendents when Get()ing a block",
	},
}

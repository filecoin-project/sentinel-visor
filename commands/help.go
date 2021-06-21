package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var HelpCmd = &cli.Command{
	Name:      "help",
	Aliases:   []string{"h"},
	Usage:     "Shows a list of commands or help for one command",
	ArgsUsage: "[command]",
	Action: func(c *cli.Context) error {
		args := c.Args()
		if args.Present() {
			return ShowCommandHelp(c, args.First())
		}

		_ = cli.ShowAppHelp(c)
		return nil
	},
}

func ShowCommandHelp(ctx *cli.Context, command string) error {
	if command == "" {
		cli.HelpPrinter(ctx.App.Writer, cli.SubcommandHelpTemplate, ctx.App)
		return nil
	}

	for _, c := range ctx.App.Commands {
		if c.HasName(command) {
			templ := c.CustomHelpTemplate
			if templ == "" {
				templ = cli.CommandHelpTemplate
			}

			cli.HelpPrinter(ctx.App.Writer, templ, c)

			return nil
		}
	}

	for _, t := range helpTopics {
		if t.Name == command {
			fmt.Fprintln(ctx.App.Writer, t.Text)
			return nil
		}
	}

	if ctx.App.CommandNotFound == nil {
		return cli.Exit(fmt.Sprintf("No help topic for '%v'", command), 3)
	}

	ctx.App.CommandNotFound(ctx, command)
	return nil
}

func Metadata() map[string]interface{} {
	return map[string]interface{}{
		"Topics": helpTopics,
	}
}

var AppHelpTemplate = `{{.Name}}{{if .Usage}} - {{.Usage}}{{end}}

Usage:

  {{.HelpName}} [global options] <command> [arguments...]

The commands are:
{{range .VisibleCategories}}{{if .Name}}
   {{.Name}}:{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{else}}{{range .VisibleCommands}}
   {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{end}}{{end}}

Use "{{.HelpName}} help <command>" for more information about a command.

Additional help topics:
{{range .Metadata.Topics}}
  {{.Name}}{{"\t"}}{{.Description}}{{end}}

Use "{{.HelpName}} help <topic>" for more information about that topic.
`

type helpTopic struct {
	Name        string
	Description string
	Text        string
}

// ----------------------------------------------------------------------------
//                                                            80 characters -->
var helpTopics = []helpTopic{
	{
		Name:        "overview",
		Description: "Overview of visor",
		Text: `Visor is an application for capturing on-chain state from the filecoin network.
It extracts data from the blocks and messages contained in each tipset and
captures the effects those messages have on actor states. Visor can 'watch'
the head of the filecoin chain for incoming tipsets or 'walk' the chain to
analyze historic tipsets.

A watch is intended to follow the growth of the chain and operates by
subscribing to incoming tipsets and processing them as they arrive. A
confidence level may be  specified which determines how many epochs visor
should wait before processing the tipset. This is to allow for chain
reorganisation near the head. A low confidence level risks extracting data from
tipsets that do not form part of the consensus chain.

A walk takes a range of heights and will walk from the heaviest tipset at the
upper height to the lower height using the parent state root present in each
tipset.

The type of data extracted by visor is controlled by 'tasks' that focus on
particular parts of the chain. For more information about available tasks
see 'visor help tasks'.

Data is extracted into models that represent chain objects, components of actor
state and derived statistics. Visor can insert these extracted models into a
TimescaleDB database as separate tables or emit them as csv files. For more
information on the database schema used by visor see 'visor help schema'.

Visor requires access to a filecoin blockstore that holds the state of the
chain. For watching incoming tipsets the blockstore must be connected and in
sync with the filecoin network. Historic walks can be performed against an
offline store. Visor may be operated in two different modes that control the
way in which the blockstore is accessed.

When run as a daemon, visor will maintain its own local blockstore and attempt
to synchronise it with the filecoin network. For more information on running
visor as a daemon, including how to initialise the blockstore, see
'visor help daemon'.

In standalone mode visor uses an external source of filecoin block data such as
a Lotus node or a chain export file. See 'visor help run' for more information.
`,
	},

	{
		Name:        "tasks",
		Description: "Available task types",
		Text: `Visor provides several tasks to capture different aspects of chain state.
The walk and watch subcommands can be configured to run specific tasks
using the --tasks option which expects a comma separated list of task
names.

General tasks that capture data present in on-chain tipsets:

  blocks          Captures data about blocks and their relationships. Populates
                  the block_headers, block_parents and drand_block_entries
                  models.

  chaineconomics  Reads circulating supply information. Populates
                  the chain_economics model.

  messages        Captures data about messages that were carried in a tipset's
                  blocks. It is possible for the same message to appear in
                  multiple blocks within a single tipset. The block_messages
                  model captures the relationship between a message and the
                  blocks it appears in. Message parameters are parsed and
                  serialized as JSON in the parsed_messages model.

                  The receipt is also captured for any messages that
                  were executed.

                  Detailed information about gas usage by each message is
                  captured in the derived_gas_outputs model. A summary of
                  gas usage by all messages in the tipset is calculated
                  and emitted in the message_gas_economy model.

                  The task does not produce any data until it has seen two
                  tipsets since receipts are carried in the tipset following
                  the one containing the messages.

Tasks for capturing actor state changes. These tasks operate by performing a diff
of an actor's state between two sequential tipsets:

  actorstatesraw       Captures basic actor properties for any actors that have
                       changed state and serializes a shallow form of the new
                       state to JSON. Populates the actors and actor_states
                       models.

  actorstatesinit      Captures changes to the init actor to provide mappings
                       between canonical ID-addresses and temporary actor
                       addresses or public keys. Populates the id_addresses model.

  actorstatesmarket    Captures new deal proposals and changes to deal states
                       recorded by the storage market actor. Populates the
                       market_deal_proposals and market_deal_states models

  actorstatesminer     Captures changes to miner actors to provide information
                       about sectors, posts and locked funds. Populates the
                       miner_current_deadline_infos, miner_fee_debts,
                       miner_locked_funds, miner_infos, miner_sector_posts,
                       miner_pre_commit_infos, miner_sector_infos,
                       miner_sector_events and miner_sector_deals models.

  actorstatesmultisig  Analyzes changes to multisig actors to capture data about
                       multisig transactions. Populates the multisig_transactions
                       model.

  actorstatespower     Analyzes changes to the storage power to capture information
                       about total power at each epoch and updates to miner power
                       claims. Populates the chain_powers and power_actor_claims
                       models.

  actorstatesreward    Captures changes in the reward actor state to provide
                       information about miner rewards for each epoch. Populates
                       the chain_rewards model.

Other tasks:

  msapprovals  Captures approvals of multisig actors by interpreting the outcome
               of approval messages sent on chain. Populates the
               multisig_approvals model.
`,
	},

	{
		Name:        "monitoring",
		Description: "Monitoring visor operation",
		Text: `Visor may be monitored during operation using logfiles, metrics and tracing.
The visor command reocgnizes environment variables and provides options to
control the behaviour of each type of monitoring output. Options should be
supplied before any sub command:

  visor [global options] <command>

Visor uses the IPFS logging library (https://github.com/ipfs/go-ipfs) to write
application logs. By default logs are written to STDERR in JSON format. Log
lines are labeled with one of seven levels to indicate severity of the message
(DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL). Line are also labeled with
named systems which indicate the area of function that produced the log. Each
system may be configured to only emit log messages of a specific level or
higher. By default all log levels are set to debug and above.

A number of environment variables may be used to control the format and
destination of the logs.

  GOLOG_LOG_LEVEL        Set the default log level for all log systems.

  GOLOG_LOG_FMT          Set the output log format. By default logs will be
                         colorized and text format. Use 'json' to specify
                         JSON formatted logs and 'nocolor' to log in text
                         format without colors.

  GOLOG_FILE             Specify the name of the file that logs should be
                         written to. Only used if GOLOG_OUTPUT contains the
                         'file' keyword.

  GOLOG_OUTPUT           Specify whether to output to file, stderr, stdout or
                         a combination. Separate each keyword with a '+', for
                         example: file+stderr

  VISOR_LOG_LEVEL_NAMED  Set the log level of specific loggers. The value
                         should be a comma delimited list of log systems and
                         log levels formatted as name:level, for example
                         'logger1:debug,logger2:info'.

In addition, visor supports some global options for controlling logging:

  --log-level LEVEL        Set the default log level for all loggers to LEVEL.
                           This option overrides any value set using the
                           GOLOG_LOG_LEVEL environment variable.

  --log-level-named value  Set the log level of specific loggers. This option
                           overrides any value set using the
                           VISOR_LOG_LEVEL_NAMED environment variable.

To control logging output while the visor daemon is running see 'visor help log'.

During operation visor exposes metrics and debugging information on port 9991
by default. The address used by this http server can be changed using the
'--prometheus-port' option which expects an IP address and port number. The
address may be omitted to run the server on all interfaces, for example: ':9991'.

The following paths can be accessed using a standard web browser.

  /metrics       Metrics published in prometheus format

  /debug/pprof/  Access to standard Go profiling and debugging information
                 memory allocations, cpu profile and active goroutines dumps.

Visor can publish function level tracing to a Jaeger compatible service. By
default tracing is disabled.

Environment variables for controlling function level tracing:

  VISOR_TRACING         Enable tracing. Set to 'true' to enable tracing.

  JAEGER_AGENT_HOST,    Hostname and port of a Jaeger compatible agent that
  JAEGER_AGENT_PORT     visor should send traces to.

  JAEGER_SERVICE_NAME   The name visor should use when reporting traces.

  JAEGER_SAMPLER_TYPE,  Control the type of sampling used to capture traces.
  JAEGER_SAMPLER_PARAM  The type may be either 'const' or 'probabilistic'.
                        The behaviour of the sampler is controlled by the
                        value of param. For a 'const' sampler a value of 1
                        indicates that every function call should be traced,
                        while 0 means none should be traced. No intermediate
                        values are accepted. For a 'probabilistic' sampler
                        the param indicates the fraction of function calls
                        that should be sampled.

The following options may be used to override the tracing environment variables:

  --tracing
  --jaeger-agent-host
  --jaeger-agent-port
  --jaeger-service-name
  --jaeger-sampler-type
  --jaeger-sampler-param
`,
	},
}

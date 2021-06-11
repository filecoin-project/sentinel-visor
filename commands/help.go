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

GLOBAL OPTIONS:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}
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
		Name:        "tasks",
		Description: "available task types",
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
                       about total power at each epoch and updates to miner power claims.
                       Populates the chain_powers and power_actor_claims models.

  actorstatesreward    Captures changes in the reward actor state to provide
                       information about miner rewards for each epoch. Populates
                       the chain_rewards model.

Other tasks:

  msapprovals  Captures approvals of multisig actors by interpreting the outcome
               of approval messages sent on chain. Populates the
               multisig_approvals model.
`,
	},
}

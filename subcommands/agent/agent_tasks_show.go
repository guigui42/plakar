package agent

import (
	"flag"
	"fmt"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands"
)

type AgentTasksShow struct {
	subcommands.SubcommandBase
}

func (cmd *AgentTasksShow) Parse(ctx *appcontext.AppContext, args []string) error {
	flags := flag.NewFlagSet("agent tasks", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [OPTIONS]\n", flags.Name())
		fmt.Fprintf(flags.Output(), "\nOPTIONS:\n")
		flags.PrintDefaults()
	}
	flags.Parse(args)

	if flags.NArg() != 0 {
		return fmt.Errorf("too many arguments")
	}

	return nil
}

func (cmd *AgentTasksShow) Execute(ctx *appcontext.AppContext, repo *repository.Repository) (int, error) {
	if agentContextSingleton == nil {
		return 1, fmt.Errorf("agent not started")
	}

	if agentContextSingleton.schedulerConfig == nil {
		return 1, fmt.Errorf("tasks not configured")
	}

	agentContextSingleton.schedulerConfig.Write(ctx.Stdout)

	return 0, nil
}

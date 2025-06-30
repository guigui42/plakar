package scheduler

import (
	"fmt"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands/check"
)

type CheckTask struct {
	TaskBase
	Cmd check.Check
}

func (task *CheckTask) Run(ctx *appcontext.AppContext, jobName string) {
	repo, store, err := task.LoadRepository(ctx)
	if err != nil {
		ctx.GetLogger().Error("Error loading repository: %s", err)
		return
	}
	defer store.Close()
	defer repo.Close()

	reporter := task.NewReporter(ctx, repo, jobName)

	retval, err := task.Cmd.Execute(ctx, repo)
	if err != nil || retval != 0 {
		ctx.GetLogger().Error("Error executing check: %s", err)
		reporter.TaskFailed(1, "Error executing check: retval=%d, err=%s", retval, err)
		return
	}

	reporter.TaskDone()
}

func (task *CheckTask) String() string {
	return fmt.Sprintf("check %s", task.Repository)
}

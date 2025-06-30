package scheduler

import (
	"fmt"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands/restore"
)

type RestoreTask struct {
	TaskBase
	Cmd restore.Restore
}

func (task *RestoreTask) Run(ctx *appcontext.AppContext, jobName string) {
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
		ctx.GetLogger().Error("Error executing restore: %s", err)
		reporter.TaskFailed(1, "Error executing restore: retval=%d, err=%s", retval, err)
		return
	}

	reporter.TaskDone()
}

func (task *RestoreTask) String() string {
	return fmt.Sprintf("restore %s to %q", task.Repository, task.Cmd.Target)
}

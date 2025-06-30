package scheduler

import (
	"time"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands/maintenance"
)

type MaintenanceTask struct {
	TaskBase
	Retention time.Duration
	Cmd       maintenance.Maintenance
}

func (task *MaintenanceTask) Run(ctx *appcontext.AppContext, jobName string) {
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
		ctx.GetLogger().Error("Error executing maintenance: %s", err)
		reporter.TaskFailed(1, "Error executing maintenance: retval=%d, err=%s", retval, err)
		return
	}

	ctx.GetLogger().Info("maintenance of repository %s succeeded", task.Repository)
	reporter.TaskDone()

	if task.Retention != 0 {
		runRmTask(ctx, task.Repository, repo, jobName, task.Retention)
	}
}

func (task *MaintenanceTask) String() string {
	return "maintenance on " + task.Repository
}

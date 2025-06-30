package scheduler

import (
	"fmt"
	"time"

	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands/rm"
	"github.com/PlakarKorp/plakar/utils"
)

type RmTask struct {
	TaskBase
	Cmd       rm.Rm
	Retention time.Duration
}

func (task *RmTask) Run(ctx *appcontext.AppContext, jobName string) {
	repo, store, err := task.LoadRepository(ctx)
	if err != nil {
		ctx.GetLogger().Error("Error loading repository: %s", err)
		return
	}
	defer store.Close()
	defer repo.Close()

	task.Run2(ctx, jobName, repo)
}

func (task *RmTask) Run2(ctx *appcontext.AppContext, jobName string, repo *repository.Repository) {

	reporter := task.NewReporter(ctx, repo, jobName)

	if task.Cmd.LocateOptions == nil {
		task.Cmd.LocateOptions = utils.NewDefaultLocateOptions()
	}
	task.Cmd.LocateOptions.Job = jobName
	task.Cmd.LocateOptions.Before = time.Now().Add(-task.Retention)

	if retval, err := task.Cmd.Execute(ctx, repo); err != nil || retval != 0 {
		ctx.GetLogger().Error("Error removing snapshots: %s", err)
		reporter.TaskFailed(1, "Error removing snapshots: retval=%d, err=%s", retval, err)
		return
	}

	reporter.TaskDone()
}

func (task *RmTask) String() string {
	return fmt.Sprintf("rm on %s", task.Repository)
}

func runRmTask(ctx *appcontext.AppContext, repoName string, repo *repository.Repository, jobName string, duration time.Duration) {
	task := &RmTask{
		TaskBase: TaskBase{
			Repository: repoName,
			Type:       "RM",
		},
		Retention: duration,
	}
	task.Run2(ctx, jobName, repo)
}

package scheduler

import (
	"fmt"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands/sync"
)

type SyncTask struct {
	TaskBase
	Kloset string
	Cmd    sync.Sync
}

func (task *SyncTask) Run(ctx *appcontext.AppContext, jobName string) {

	repo, store, err := task.LoadRepository(ctx)
	if err != nil {
		ctx.GetLogger().Error("Error loading repository: %s", err)
		return
	}
	defer store.Close()
	defer repo.Close()

	reporter := task.NewReporter(ctx, repo, jobName)

	task.Cmd.PeerRepositoryLocation = task.Kloset // XXX

	retval, err := task.Cmd.Execute(ctx, repo)
	if err != nil || retval != 0 {
		ctx.GetLogger().Error("sync: %s", err)
		reporter.TaskFailed(1, "Error executing sync: retval=%d, err=%s", retval, err)
		return
	}

	ctx.GetLogger().Info("sync: synchronization succeeded")
	reporter.TaskDone()
}

func (task *SyncTask) String() string {
	return fmt.Sprintf("sync %s %s %s", task.Repository, task.Cmd.Direction, task.Kloset)
}

package scheduler

import (
	"fmt"
	"time"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands/backup"
)

type BackupTask struct {
	TaskBase
	Retention time.Duration
	Cmd       backup.Backup
}

func (task *BackupTask) Run(ctx *appcontext.AppContext, jobName string) {
	repo, store, err := task.LoadRepository(ctx)
	if err != nil {
		ctx.GetLogger().Error("Error loading repository: %s", err)
		return
	}
	defer store.Close()
	defer repo.Close()

	reporter := task.NewReporter(ctx, repo, jobName)

	task.Cmd.Job = jobName
	retval, err, snapId, reportWarning := task.Cmd.DoBackup(ctx, repo)
	if err != nil || retval != 0 {
		ctx.GetLogger().Error("Error creating backup: %s", err)
		reporter.TaskFailed(1, "Error creating backup: retval=%d, err=%s", retval, err)
		return
	}

	reporter.WithSnapshotID(snapId)
	if reportWarning != nil {
		reporter.TaskWarning("Warning during backup: %s", reportWarning)
	} else {
		reporter.TaskDone()
	}

	if task.Retention != 0 {
		runRmTask(ctx, task.Repository, repo, jobName, task.Retention)
	}
}

func (task *BackupTask) String() string {
	return fmt.Sprintf("backup %s on %s", task.Cmd.Path, task.Repository)
}

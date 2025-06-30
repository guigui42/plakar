package scheduler

import (
	"sync"
	"time"

	"github.com/PlakarKorp/plakar/appcontext"
)

type Job struct {
	Name      string
	Task      Task
	Schedules []Schedule

	isRunning     bool
	lastRun       time.Time
	lastActualRun time.Time

	mu sync.Mutex
}

type ScheduledJob struct {
	event     *Event[*ScheduledJob]
	scheduled time.Time
	job       *Job
}

func (s *ScheduledJob) Execute(ctx *appcontext.AppContext) {
	s.job.mu.Lock()

	// Do not execute a job if the previous invocation is sill running.
	if s.job.isRunning {
		s.job.mu.Unlock()
		ctx.GetLogger().Warn("job %q: still running", s.job.Name)
		return
	}

	delay := time.Since(s.scheduled)
	if delay > 5*time.Second {
		// This might happen if the machie was suspended.
		ctx.GetLogger().Warn("job %q: overdue by %s", s.job.Name, delay)
	}

	s.job.mu.Unlock()
	s.job.lastRun = s.scheduled
	s.job.lastActualRun = time.Now()
	go func() {
		ctx.GetLogger().Info("job %q: running", s.job.Name)
		nctx := appcontext.NewAppContextFrom(ctx)
		s.job.Task.Run(nctx, s.job.Name)
		ctx.GetLogger().Info("job %q: done", s.job.Name)
		s.job.isRunning = false // lock is not needed here
	}()
}

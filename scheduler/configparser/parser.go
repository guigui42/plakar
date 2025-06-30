package configparser

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/PlakarKorp/plakar/scheduler"
)

type ConfigParser struct {
	buf       []byte
	pos       int
	tokenPos  int
	lineno    int
	reader    io.Reader
	reporting bool
	config    *scheduler.Configuration
	err       error
	lexerr    error

	currentTask scheduler.Task
}

func ParseConfig(reader io.Reader) (*scheduler.Configuration, error) {
	cfg := scheduler.Configuration{}
	cfg.Jobs = make(map[string]*scheduler.Job)
	parser := ConfigParser{
		reader: reader,
		config: &cfg,
		pos:    -1,
	}
	var err error

	ret := yyParse(&parser)
	if ret != 0 {
		lineno, col, line, _ := parser.findLine(parser.tokenPos)
		fmt.Fprintf(os.Stderr, "line %d, column %d: %v\n", lineno, col, parser.err)
		fmt.Fprintf(os.Stderr, "%s\n", line)
		fmt.Fprintf(os.Stderr, "%s^\n", strings.Repeat(" ", col))

		err = parser.err
		if parser.lexerr != nil {
			err = parser.lexerr
		}
		return nil, fmt.Errorf("Parse error: %v", err)
	}

	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ParseConfigString(s string) (*scheduler.Configuration, error) {
	return ParseConfig(strings.NewReader(s))
}

func ParseConfigFile(path string) (*scheduler.Configuration, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ParseConfig(file)
}

func (parser *ConfigParser) PushJob(name string, task scheduler.Task, schedules []scheduler.Schedule) {
	if task != parser.currentTask {
		panic("task mismatch")
	}
	parser.config.Jobs[name] = &scheduler.Job{
		Name:      name,
		Task:      task,
		Schedules: schedules,
	}
}

func (parser *ConfigParser) HasJob(name string) bool {
	_, ok := parser.config.Jobs[name]
	return ok
}

func (parser *ConfigParser) MakeBackupTask(repo string, source string) scheduler.Task {
	task := &scheduler.BackupTask{
		TaskBase: scheduler.TaskBase{
			Repository: repo,
			Type:       "BACKUP",
			Reporting:  parser.reporting,
		},
	}
	task.Cmd.Path = source
	task.Cmd.Opts = make(map[string]string)
	task.Cmd.Quiet = true
	parser.currentTask = task
	return parser.currentTask
}

func (parser *ConfigParser) MakeCheckTask(repo string) scheduler.Task {
	parser.currentTask = &scheduler.CheckTask{
		TaskBase: scheduler.TaskBase{
			Repository: repo,
			Type:       "CHECK",
			Reporting:  parser.reporting,
		},
	}
	return parser.currentTask
}

func (parser *ConfigParser) MakeMaintenanceTask(repo string) scheduler.Task {
	parser.currentTask = &scheduler.MaintenanceTask{
		TaskBase: scheduler.TaskBase{
			Repository: repo,
			Type:       "MAINTENANCE",
			Reporting:  parser.reporting,
		},
	}
	return parser.currentTask
}

func (parser *ConfigParser) MakeRestoreTask(repo string, destination string) scheduler.Task {
	task := &scheduler.RestoreTask{
		TaskBase: scheduler.TaskBase{
			Repository: repo,
			Type:       "RESTORE",
			Reporting:  parser.reporting,
		},
	}
	task.Cmd.Target = destination
	task.Cmd.Quiet = true
	parser.currentTask = task
	return parser.currentTask
}

func (parser *ConfigParser) MakeSyncTask(repo string, direction string, kloset string) scheduler.Task {
	task := &scheduler.SyncTask{
		TaskBase: scheduler.TaskBase{
			Repository: repo,
			Type:       "SYNC",
			Reporting:  parser.reporting,
		},
		Kloset: kloset,
	}
	task.Cmd.Direction = direction
	parser.currentTask = task
	return parser.currentTask
}

func (parser *ConfigParser) MakeScheduleAt(moments []scheduler.Time) scheduler.Schedule {
	return &scheduler.ScheduleAt{
		At: moments,
	}
}

func (parser *ConfigParser) MakeScheduleEvery(period time.Duration, from, until scheduler.Time) scheduler.Schedule {
	return &scheduler.ScheduleEvery{
		Period: period,
		From:   from,
		Until:  until,
	}
}

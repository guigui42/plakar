package scheduler

import (
	"fmt"
	"strings"

	"github.com/PlakarKorp/kloset/encryption"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/storage"
	"github.com/PlakarKorp/kloset/versioning"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/reporting"
	"github.com/PlakarKorp/plakar/services"
)

type Task interface {
	Run(ctx *appcontext.AppContext, jobName string)
	String() string
}

type TaskBase struct {
	Type       string
	Repository string
	Reporting  bool
}

func (t *TaskBase) NewReporter(ctx *appcontext.AppContext, repo *repository.Repository, taskName string) *reporting.Reporter {
	doReport := false
	if t.Reporting {
		doReport = true
		authToken, err := ctx.GetAuthToken(repo.Configuration().RepositoryID)
		if err != nil || authToken == "" {
			doReport = false
		} else {
			sc := services.NewServiceConnector(ctx, authToken)
			enabled, err := sc.GetServiceStatus("alerting")
			if err != nil || !enabled {
				doReport = false
			}
		}
	}

	reporter := reporting.NewReporter(ctx, doReport, repo, ctx.GetLogger())
	reporter.TaskStart(strings.ToLower(t.Type), taskName)
	reporter.WithRepositoryName(t.Repository)
	reporter.WithRepository(repo)
	return reporter
}

func (t *TaskBase) LoadRepository(ctx *appcontext.AppContext) (*repository.Repository, storage.Store, error) {
	storeConfig, err := ctx.Config.GetRepository(t.Repository)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get repository configuration: %w", err)
	}

	store, config, err := storage.Open(ctx.GetInner(), storeConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open storage: %w", err)
	}

	repoConfig, err := storage.NewConfigurationFromWrappedBytes(config)
	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("unable to read repository configuration: %w", err)
	}

	if repoConfig.Version != versioning.FromString(storage.VERSION) {
		store.Close()
		return nil, nil, fmt.Errorf("incompatible repository version: %s != %s", repoConfig.Version, storage.VERSION)
	}

	var key []byte
	if passphrase, ok := storeConfig["passphrase"]; ok {
		key, err = encryption.DeriveKey(repoConfig.Encryption.KDFParams, []byte(passphrase))
		if err != nil {
			store.Close()
			return nil, nil, fmt.Errorf("error deriving key: %w", err)
		}
		if !encryption.VerifyCanary(repoConfig.Encryption, key) {
			store.Close()
			return nil, nil, fmt.Errorf("invalid passphrase")
		}
	}

	repo, err := repository.New(ctx.GetInner(), key, store, config)
	if err != nil {
		store.Close()
		return nil, store, fmt.Errorf("unable to open repository: %w", err)
	}
	return repo, store, nil
}

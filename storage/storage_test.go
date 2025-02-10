package storage_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/logging"
	"github.com/PlakarKorp/plakar/storage"
	ptesting "github.com/PlakarKorp/plakar/testing"
)

func TestNewStore(t *testing.T) {
	ctx := appcontext.NewAppContext()
	ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
	ctx.MaxConcurrency = runtime.NumCPU()*8 + 1

	store, err := storage.NewStore("fs", "/test/location")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if store.Location() != "/test/location" {
		t.Errorf("expected location to be '/test/location', got %v", store.Location())
	}

	// should return an error as the backend does not exist
	_, err = storage.NewStore("unknown", "/test/location")
	if err.Error() != "backend 'unknown' does not exist" {
		t.Fatalf("Expected %s but got %v", "backend 'unknown' does not exist", err)
	}
}

func TestCreateStore(t *testing.T) {
	ctx := appcontext.NewAppContext()
	ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
	ctx.MaxConcurrency = runtime.NumCPU()*8 + 1

	config := storage.NewConfiguration()
	serializedConfig, err := config.ToBytes()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = storage.Create("/test/location", serializedConfig)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// should return an error as the backend Create will return an error
	_, err = storage.Create("/test/location/musterror", serializedConfig)
	if err.Error() != "creating error" {
		t.Fatalf("Expected %s but got %v", "opening error", err)
	}

	// should return an error as the backend does not exist
	_, err = storage.Create("unknown://dummy", serializedConfig)
	if err.Error() != "unsupported plakar protocol" {
		t.Fatalf("Expected %s but got %v", "unsupported plakar protocol", err)
	}
}

func TestOpenStore(t *testing.T) {
	ctx := appcontext.NewAppContext()
	ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
	ctx.MaxConcurrency = runtime.NumCPU()*8 + 1

	store, _, err := storage.Open("/test/location")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if store.Location() != "/test/location" {
		t.Errorf("expected location to be '/test/location', got %v", store.Location())
	}

	// should return an error as the backend Open will return an error
	_, _, err = storage.Open("/test/location/musterror")
	if err.Error() != "opening error" {
		t.Fatalf("Expected %s but got %v", "opening error", err)
	}

	// should return an error as the backend does not exist
	_, _, err = storage.Open("unknown://dummy")
	if err.Error() != "unsupported plakar protocol" {
		t.Fatalf("Expected %s but got %v", "unsupported plakar protocol", err)
	}
}

func TestBackends(t *testing.T) {
	ctx := appcontext.NewAppContext()
	ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
	ctx.MaxConcurrency = runtime.NumCPU()*8 + 1

	storage.Register("test", func(location string) storage.Store { return &ptesting.MockBackend{} })

	expected := []string{"fs", "test"}
	actual := storage.Backends()
	if len(expected) != len(actual) {
		t.Errorf("expected %d backends, got %d", len(expected), len(actual))
	}
}

func TestNew(t *testing.T) {
	locations := []struct {
		name     string
		location string
	}{
		{"fs2", "fs://test/location"},
		{"plakard", "ssh://test/location"},
		{"http", "http://test/location"},
		{"database", "sqlite:///test/location"},
		{"s3", "s3://test/location"},
		{"null", "null://test/location"},
	}

	for _, l := range locations {
		t.Run(l.name, func(t *testing.T) {
			ctx := appcontext.NewAppContext()
			ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
			ctx.MaxConcurrency = runtime.NumCPU()*8 + 1

			storage.Register(l.name, func(location string) storage.Store { return ptesting.NewMockBackend(location) })

			store, err := storage.New(l.location)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if store.Location() != l.location {
				t.Errorf("expected location to be '%s', got %v", l.location, store.Location())
			}
		})
	}

	t.Run("unknown backend", func(t *testing.T) {
		ctx := appcontext.NewAppContext()
		ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
		ctx.MaxConcurrency = runtime.NumCPU()*8 + 1

		// storage.Register("unknown", func(location string) storage.Store { return ptesting.NewMockBackend(location) })
		_, err := storage.New("unknown://dummy")
		if err.Error() != "unsupported plakar protocol" {
			t.Fatalf("Expected %s but got %v", "unsupported plakar protocol", err)
		}
	})

	t.Run("absolute fs path", func(t *testing.T) {
		ctx := appcontext.NewAppContext()
		ctx.SetLogger(logging.NewLogger(os.Stdout, os.Stderr))
		ctx.MaxConcurrency = runtime.NumCPU()*8 + 1

		// storage.Register("unknown", func(location string) storage.Store { return ptesting.NewMockBackend(location) })
		store, err := storage.New("dummy")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if store.Location() != cwd+"/dummy" {
			t.Errorf("expected location to be '%s', got %v", "dummy", store.Location())
		}
	})
}

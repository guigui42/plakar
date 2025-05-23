package viewers

import (
	_ "embed"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PlakarKorp/kloset/appcontext"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/subcommands/server"
)

//go:embed volume.yaml
var volumesCompose string

type Viewer interface {
	GetCompose() string
}

type AttachableViewer interface {
	// Attach returns the name of the container to attach to.
	Attach() string
}

type Runner struct {
	Path string

	viewer Viewer
	repo   *repository.Repository
}

func NewRunner(repo *repository.Repository, viewer Viewer) (*Runner, error) {
	// Create a temporary directory to store the compose file
	tmpDir, err := os.MkdirTemp("", "plakar-viewer-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	composePath := filepath.Join(tmpDir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte(viewer.GetCompose()), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to write compose file: %w", err)
	}

	return &Runner{
		Path:   tmpDir,
		viewer: viewer,
		repo:   repo,
	}, nil
}

func (r *Runner) Run(ctx *appcontext.AppContext, snapshot string, path string) error {
	// Run Plakar HTTP server in a goroutine. Necessary for visualization
	// XXX: listen on a random port instead, or even on a UNIX socket

	min := 9000
	max := 12000
	port := rand.Intn(max-min+1) + min // random int in [9000, 12000]

	go func() {
		server := &server.Server{
			ListenAddr: fmt.Sprintf("127.0.0.1:%d", port),
			NoDelete:   true,
		}
		// XXX: find a way to kill the server on cleanup
		_, _ = server.Execute(ctx, r.repo)
	}()

	// Create the volume.yaml file
	content := strings.ReplaceAll(volumesCompose, "__PLAKAR_SERVER__", fmt.Sprintf("http://host.docker.internal:%d", port))
	content = strings.ReplaceAll(content, "__PLAKAR_SNAPSHOT__", snapshot)
	content = strings.ReplaceAll(content, "__PLAKAR_SNAPSHOT_PATH__", path)
	composePath := filepath.Join(r.Path, "volumes.yaml")

	if err := os.WriteFile(composePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	cmd := exec.Command(
		"docker", "compose",
		"-f", filepath.Join(r.Path, "compose.yaml"),
		"-f", filepath.Join(r.Path, "volumes.yaml"),
		"up", "-d",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = r.Path // necessary so Compose uses that directory as its context

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run docker compose: %w", err)
	}

	return nil
}

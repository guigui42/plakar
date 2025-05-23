package viewers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

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
}

func NewRunner(viewer Viewer) (*Runner, error) {
	// Create a temporary directory to store the compose file
	tmpDir, err := os.MkdirTemp("", "plakar-viewer-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(viewer.GetCompose()), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to write compose file: %w", err)
	}

	return &Runner{
		Path:   tmpDir,
		viewer: viewer,
	}, nil
}

func (r *Runner) Run() error {
	cmd := exec.Command("docker", "compose", "-f", filepath.Join(r.Path, "docker-compose.yml"), "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = r.Path // necessary so Compose uses that directory as its context

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run docker compose: %w", err)
	}

	return nil
}

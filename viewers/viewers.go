package viewers

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creack/pty"

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
	GetExecArgs() []string
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

type RunnerStatus struct {
	Services   []string
	Attachable bool
}

// DockerComposePsOutput is the output of the `docker compose ps --format json`
// command. The command actually returns more fields than this, but we don't
// need them.
type DockerComposePsOutput struct {
	Publishers []struct {
		URL           string `json:"URL"`
		PublishedPort int    `json:"PublishedPort"`
	}
}

func (r *Runner) Run(ctx *appcontext.AppContext, snapshot string, path string) (*RunnerStatus, error) {
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
		return nil, fmt.Errorf("failed to write compose file: %w", err)
	}

	upCmd := exec.Command(
		"docker", "compose",
		"-f", filepath.Join(r.Path, "compose.yaml"),
		"-f", filepath.Join(r.Path, "volumes.yaml"),
		"up", "-d",
	)
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	upCmd.Dir = r.Path // necessary so Compose uses that directory as its context

	if err := upCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run docker compose: %w", err)
	}

	psCmd := exec.Command(
		"docker", "compose",
		"-f", filepath.Join(r.Path, "compose.yaml"),
		"-f", filepath.Join(r.Path, "volumes.yaml"),
		"ps", "--format", "json",
	)

	output, err := psCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run docker compose ps: %w", err)
	}

	services := []string{}

	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}

		var psOutput DockerComposePsOutput
		if err := json.Unmarshal([]byte(line), &psOutput); err != nil {
			return nil, fmt.Errorf("failed to unmarshal docker compose ps output: %w", err)
		}

		for _, publisher := range psOutput.Publishers {
			services = append(services, fmt.Sprintf("%s:%d", publisher.URL, publisher.PublishedPort))
		}
	}

	_, attachable := r.viewer.(AttachableViewer)

	return &RunnerStatus{
		Services:   services,
		Attachable: attachable,
	}, nil
}

func (r *Runner) Attach() (*exec.Cmd, *os.File, error) {
	viewer, ok := r.viewer.(AttachableViewer)
	if !ok {
		return nil, nil, fmt.Errorf("viewer does not support attaching")
	}

	cmdArgs := []string{
		"compose",
		"-f", filepath.Join(r.Path, "compose.yaml"),
		"-f", filepath.Join(r.Path, "volumes.yaml"),
		"exec",
	}
	cmdArgs = append(cmdArgs, viewer.GetExecArgs()...)

	cmd := exec.Command(
		"docker", cmdArgs...,
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start pty: %w", err)
	}

	return cmd, ptmx, nil
}

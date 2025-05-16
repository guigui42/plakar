package pvr

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type PVR struct {
	mu         sync.Mutex
	containers map[string]*Container
}

type Container struct {
	Id         string
	LastAttach time.Time
}

func New() (*PVR, error) {
	d := &PVR{
		containers: make(map[string]*Container),
	}
	// XXX: pull image or build
	// d.ReattachExistingContainers()
	// go d.gcLoop()
	return d, nil
}

type Service struct {
}

func (p *PVR) ListServices() ([]Service, error) {
	services := []Service{}

	return services, nil
}

type StartContainerOptions struct {
}

func (d *PVR) StartContainer(opt StartContainerOptions) (*Container, error) {
	containerPrefix := "plakar-pvr-"
	containerName := containerPrefix + "test"

	// XXX: add repository id in the labels, we don't want to manage the containers from other repositories
	args := []string{
		"run",
		"-q",
		"--rm",
		"--privileged",
		"--name", containerName,
		"--label", "plakar.pvr=true",
		"-t",
		"-d",
		"ubuntu", // image name
		"sleep",
		"1000",
	}

	cmd := exec.Command("docker", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Read the container ID from stdout
	out := make([]byte, 128)
	n, err := stdout.Read(out)
	if err != nil {
		return nil, fmt.Errorf("failed to read container ID: %w", err)
	}

	containerId := strings.TrimSpace(string(out[:n]))

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("failed to wait for container: %w", err)
	}

	container := &Container{
		Id: containerId,
	}

	d.mu.Lock()
	d.containers[containerId] = container
	d.mu.Unlock()

	return container, nil
}

func (d *PVR) AttachToContainer(ID string) (*Container, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	container, ok := d.containers[ID]
	if !ok {
		return nil, fmt.Errorf("container not found")
	}
	container.LastAttach = time.Now()
	return container, nil
}

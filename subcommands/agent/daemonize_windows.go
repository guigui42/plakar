package agent

import (
	"os"
	"os/exec"
)

func daemonize(argv []string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{"/b", binary}
	args = append(args, argv...)
	cmd := exec.Command("start", args...)
	return cmd.Run()
}

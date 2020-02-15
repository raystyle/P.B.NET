// +build linux

package shell

import (
	"os/exec"
)

// Shell ...
func Shell(command string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)
	return cmd.CombinedOutput()
}

func createCommand(path string, args []string) *exec.Cmd {
	if path == "" {
		path = "sh"
	}
	cmd := exec.Command(path, args...)
	return cmd
}

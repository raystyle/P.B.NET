// +build linux

package shell

import (
	"os"
	"os/exec"
)

// Shell ...
func Shell(command string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command) // #nosec
	return cmd.CombinedOutput()
}

func createCommand(path string, args []string) *exec.Cmd {
	if path == "" {
		path = "sh"
	}
	return exec.Command(path, args...) // #nosec
}

func sendInterruptSignal(cmd *exec.Cmd) error {
	return cmd.Process.Signal(os.Interrupt)
}

// +build linux

package shell

import (
	"os/exec"
)

// Shell ...
func Shell(command string) ([]byte, error) {
	cmd := exec.Command("bash", "-c", command)
	return cmd.CombinedOutput()
}

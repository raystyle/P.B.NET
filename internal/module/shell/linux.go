// +build linux

package shell

import (
	"os"
	"os/exec"
	"syscall"
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
	cmd := exec.Command(path, args...) // #nosec
	cmd.SysProcAttr = setSysProcAttr()
	return cmd
}

func setSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func sendInterruptSignal(cmd *exec.Cmd) error {
	return cmd.Process.Signal(os.Interrupt)
}

// +build windows

package shell

import (
	"os/exec"
	"syscall"
)

// Shell ...
func Shell(command string) ([]byte, error) {
	cmd := exec.Command("cmd.exe", "/c", command)
	attr := syscall.SysProcAttr{
		HideWindow: true,
	}
	cmd.SysProcAttr = &attr
	return cmd.CombinedOutput()
}

func createCommand(path string, args []string) *exec.Cmd {
	if path == "" {
		path = "cmd.exe"
	}
	cmd := exec.Command(path, args...)
	attr := syscall.SysProcAttr{
		HideWindow: true,
	}
	cmd.SysProcAttr = &attr
	return cmd
}

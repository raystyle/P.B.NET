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

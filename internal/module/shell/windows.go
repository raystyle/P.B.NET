// +build windows

package shell

import (
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
)

// Shell ...
func Shell(command string) ([]byte, error) {
	cmd := exec.Command("cmd.exe", "/c", command) // #nosec
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
	cmd := exec.Command(path, args...) // #nosec
	attr := syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.SysProcAttr = &attr
	return cmd
}

// setHandler is used to hook handle interrupt signal
var setHandler uintptr

func init() {
	setHandler = syscall.NewCallback(setConsoleCtrlHandler)
}

// always return true
func setConsoleCtrlHandler(_ uintptr) uintptr {
	dll, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return uintptr(1)
	}
	proc, err := dll.FindProc("SetConsoleCtrlHandler")
	if err != nil {
		return uintptr(1)
	}
	// self delete handler
	_, _, _ = proc.Call(setHandler, uintptr(0))
	return uintptr(1)
}

func sendInterruptSignal(cmd *exec.Cmd) error {
	dll, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	// first call SetConsoleCtrlHandler, then call GenerateConsoleCtrlEvent
	// interrupt signal will not send to this process.
	// https://docs.microsoft.com/en-us/windows/console/setconsolectrlhandler
	proc, err := dll.FindProc("SetConsoleCtrlHandler")
	if err != nil {
		return err
	}
	r1, _, err := proc.Call(setHandler, uintptr(1))
	if r1 == 0 {
		return errors.Errorf("failed to call SetConsoleCtrlHandler: %s", err)
	}
	// Send the CTRL_C_EVENT to a console process group that shares
	// the console associated with the calling process.
	// https://docs.microsoft.com/en-us/windows/console/generateconsolectrlevent
	proc, err = dll.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return err
	}
	pid := cmd.Process.Pid
	r1, _, err = proc.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	if r1 == 0 {
		return errors.Errorf("failed to call CTRL_C_EVENT: %s", err)
	}
	return nil
}

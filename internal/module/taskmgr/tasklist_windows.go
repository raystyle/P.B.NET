// +build windows

package taskmgr

import (
	"github.com/StackExchange/wmi"
)

// ReturnValue about process
const (
	networkService uint32 = 2
)

type process struct {
	Name string
	PID  int64
	PPID int64

	SessionID uint32
	UserName  string // TODO finish it

	// for calculate CPU usage
	UserModeTime   uint64
	KernelModeTime uint64

	MemoryUsed uint64

	HandleCount uint32
	ThreadCount uint32

	IOWriteBytes uint64
	IOReadBytes  uint64

	CommandLine    string
	ExecutablePath string
}

type taskList struct {
	wmi wmi.Client
}

func newTaskList() (*taskList, error) {
	// client:= wmi.Client{}

	// wmi.InitializeSWbemServices()

	return nil, nil
}

func (tl *taskList) GetProcesses() ([]*Process, error) {
	return nil, nil
}

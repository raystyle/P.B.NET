// +build windows

package taskmgr

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/module/wmi"
)

var wql = wmi.BuildWQLStatement(win32Process{}, "Win32_Process")

type win32OperatingSystem struct {
	OSArchitecture string
}

type win32Process struct {
	Name            string
	ProcessID       int64
	ParentProcessID int64

	SessionID uint32

	// for calculate CPU usage
	UserModeTime   uint64
	KernelModeTime uint64

	WorkingSetSize uint64

	HandleCount uint32
	ThreadCount uint32

	ReadTransferCount  uint64
	WriteTransferCount uint64

	CommandLine    string
	ExecutablePath string
	CreationDate   time.Time
}

type win32ProcessGetOwner struct {
	Domain string
	User   string
}

type taskList struct {
	client  *wmi.Client
	lazyDLL *windows.DLL
	is64    bool
}

func newTaskList() (*taskList, error) {
	client, err := wmi.NewClient("root\\cimv2", nil)
	if err != nil {
		return nil, err
	}
	// check current operating system is 64-bit
	var sys []*win32OperatingSystem
	err = client.Query("select OSArchitecture from Win32_OperatingSystem", &sys)
	if err != nil {
		return nil, err
	}
	if len(sys) == 0 {
		return nil, errors.New("failed to get operating system architecture")
	}
	is64 := strings.Contains(sys[0].OSArchitecture, "64")
	if is64 {

	}
	tl := taskList{
		client: client,
		is64:   is64,
	}
	return &tl, nil
}

func (tl *taskList) GetProcesses() ([]*Process, error) {
	var processes []*win32Process
	err := tl.client.Query(wql, &processes)
	if err != nil {
		return nil, err
	}
	l := len(processes)
	ps := make([]*Process, l)
	for i := 0; i < l; i++ {
		ps[i] = &Process{
			Name:           processes[i].Name,
			PID:            processes[i].ProcessID,
			PPID:           processes[i].ParentProcessID,
			SessionID:      processes[i].SessionID,
			UserModeTime:   processes[i].UserModeTime,
			KernelModeTime: processes[i].KernelModeTime,
			MemoryUsed:     processes[i].WorkingSetSize,
			HandleCount:    processes[i].HandleCount,
			ThreadCount:    processes[i].ThreadCount,
			IOReadBytes:    processes[i].ReadTransferCount,
			IOWriteBytes:   processes[i].WriteTransferCount,
			CommandLine:    processes[i].CommandLine,
			ExecutablePath: processes[i].ExecutablePath,
			CreationDate:   processes[i].CreationDate,
		}
		// get username
		path := fmt.Sprintf("Win32_Process.Handle=\"%d\"", processes[i].ProcessID)
		output := win32ProcessGetOwner{}
		err = tl.client.ExecMethod(path, "GetOwner", nil, &output)
		if err != nil {
			return nil, err
		}
		if output.Domain != "" && output.User != "" {
			ps[i].Username = output.Domain + "\\" + output.User
		} else {
			ps[i].Username = output.Domain + output.User
		}
		// get architecture

	}
	return ps, nil
}

func (tl *taskList) Close() {
	tl.client.Close()
}

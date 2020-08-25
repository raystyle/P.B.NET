// +build windows

package taskmgr

import (
	"strings"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/module/windows/wmi"
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

type taskList struct {
	client  *wmi.Client
	isWow64 *windows.LazyProc
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
	tl := taskList{
		client: client,
	}
	if strings.Contains(sys[0].OSArchitecture, "64") {
		// load proc from kernel DLL
		modKernel32 := windows.NewLazySystemDLL("kernel32.dll")
		proc := modKernel32.NewProc("IsWow64Process")
		err = proc.Find()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		tl.isWow64 = proc
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
		handle, err := openProcess(ps[i].PID)
		if err != nil {
			continue
		}
		ps[i].Username = getProcessUsername(handle)
		// get process architecture
		if tl.isWow64 != nil {
			ps[i].Architecture = tl.getProcessArchitecture(handle)
		} else {
			ps[i].Architecture = "32"
		}
		err = windows.CloseHandle(handle)
		if err != nil {
			return nil, err
		}
	}
	return ps, nil
}

func (tl *taskList) getProcessArchitecture(handle windows.Handle) string {
	var wow64 bool
	ret, _, _ := tl.isWow64.Call(uintptr(handle), uintptr(unsafe.Pointer(&wow64)))
	if ret != 0 {
		if wow64 {
			return "x86"
		}
		return "x64"
	}
	return ""
}

func (tl *taskList) Close() {
	tl.client.Close()
}

// first use query_limit, if failed use query.
func openProcess(pid int64) (windows.Handle, error) {
	p := uint32(pid)
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, p)
	if err == nil {
		return handle, nil
	}
	return windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, p)
}

func getProcessUsername(handle windows.Handle) string {
	var t windows.Token
	err := windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &t)
	if err != nil {
		return ""
	}
	tu, err := t.GetTokenUser()
	if err != nil {
		return ""
	}
	account, domain, _, err := tu.User.Sid.LookupAccount("")
	if err != nil {
		return ""
	}
	return domain + "\\" + account
}

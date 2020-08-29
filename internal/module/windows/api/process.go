// +build windows

package api

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	sizeofProcessEntry32 = uint32(unsafe.Sizeof(windows.ProcessEntry32{}))
)

// ProcessBasicInfo contains process basic information.
type ProcessBasicInfo struct {
	Name              string
	PID               uint32
	PPID              uint32
	Threads           uint32
	PriorityClassBase int32
}

// GetProcessList is used to get process list that include PiD and name.
func GetProcessList() ([]*ProcessBasicInfo, error) {
	const name = "GetProcessList"
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, newAPIError(name, "failed to create process snapshot", err)
	}
	defer func() { _ = windows.Close(snapshot) }()
	processes := make([]*ProcessBasicInfo, 0, 64)
	processEntry := &windows.ProcessEntry32{
		Size: sizeofProcessEntry32,
	}
	err = windows.Process32First(snapshot, processEntry)
	if err != nil {
		return nil, newAPIError(name, "failed to call Process32First", err)
	}
	for {
		processes = append(processes, &ProcessBasicInfo{
			Name:              windows.UTF16ToString(processEntry.ExeFile[:]),
			PID:               processEntry.ProcessID,
			PPID:              processEntry.ParentProcessID,
			Threads:           processEntry.Threads,
			PriorityClassBase: processEntry.PriClassBase,
		})
		err = windows.Process32Next(snapshot, processEntry)
		if err != nil {
			if err.(syscall.Errno) == windows.ERROR_NO_MORE_FILES {
				break
			}
			return nil, newAPIError(name, "failed to call Process32Next", err)
		}
	}
	return processes, nil
}

// GetProcessIDByName is used to get PID by process name.
func GetProcessIDByName(n string) ([]uint32, error) {
	const name = "GetProcessIDByName"
	processes, err := GetProcessList()
	if err != nil {
		return nil, newAPIError(name, "failed to get process list", err)
	}
	pid := make([]uint32, 0, 1)
	for _, process := range processes {
		if process.Name == n {
			pid = append(pid, process.PID)
		}
	}
	if len(pid) == 0 {
		return nil, newAPIErrorf(name, nil, "%q is not found", n)
	}
	return pid, nil
}

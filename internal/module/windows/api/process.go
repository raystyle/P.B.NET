// +build windows

package api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// consts about process
const (
	// 0xFFF is for Windows Server 2003 or Windows XP.
	ProcessAllAccess = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0xFFF
)

// IsWow64Process is used to check x86 program is running in the x64 system.
func IsWow64Process(hProcess windows.Handle) (bool, error) {
	const name = "IsWow64Process"
	var isWow64 bool
	err := windows.IsWow64Process(hProcess, &isWow64)
	if err != nil {
		return false, newError(name, err, "failed to check is wow64 process")
	}
	return isWow64, nil
}

// ProcessBasicInfo contains process basic information.
type ProcessBasicInfo struct {
	Name              string
	PID               uint32
	PPID              uint32
	Threads           uint32
	PriorityClassBase int32
}

// GetProcessList is used to get process list that include PiD and name. // #nosec
func GetProcessList() ([]*ProcessBasicInfo, error) {
	const name = "GetProcessList"
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, newError(name, err, "failed to create process snapshot")
	}
	defer CloseHandle(snapshot)
	processes := make([]*ProcessBasicInfo, 0, 64)
	processEntry := &windows.ProcessEntry32{
		Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{})),
	}
	err = windows.Process32First(snapshot, processEntry)
	if err != nil {
		return nil, newError(name, err, "failed to call Process32First")
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
			if err.(windows.Errno) == windows.ERROR_NO_MORE_FILES {
				break
			}
			return nil, newError(name, err, "failed to call Process32Next")
		}
	}
	return processes, nil
}

// GetProcessIDByName is used to get PID by process name.
func GetProcessIDByName(n string) ([]uint32, error) {
	const name = "GetProcessIDByName"
	processes, err := GetProcessList()
	if err != nil {
		return nil, newError(name, err, "failed to get process list")
	}
	pid := make([]uint32, 0, 1)
	for _, process := range processes {
		if process.Name == n {
			pid = append(pid, process.PID)
		}
	}
	if len(pid) == 0 {
		return nil, newErrorf(name, nil, "process \"%s\" is not found", n)
	}
	return pid, nil
}

// OpenProcess is used to open process by PID and return process handle.
func OpenProcess(desiredAccess uint32, inheritHandle bool, pid uint32) (windows.Handle, error) {
	const name = "OpenProcess"
	hProcess, err := windows.OpenProcess(desiredAccess, inheritHandle, pid)
	if err != nil {
		return 0, newErrorf(name, err, "failed to open process with PID %d", pid)
	}
	return hProcess, nil
}

// information class about NTQueryInformationProcess.
const (
	InfoClassProcessBasicInformation     uint8 = 0
	InfoClassProcessDebugPort            uint8 = 7
	InfoClassProcessWow64Information     uint8 = 26
	InfoClassProcessImageFileName        uint8 = 27
	InfoClassProcessBreakOnTermination   uint8 = 29
	InfoClassProcessSubsystemInformation uint8 = 75
)

// ProcessBasicInformation is an equivalent representation of
// PROCESS_BASIC_INFORMATION in the Windows API.
type ProcessBasicInformation struct {
	ExitStatus                   uintptr
	PEBBaseAddress               uintptr
	AffinityMask                 uintptr
	BasePriority                 uintptr
	UniqueProcessID              *uint32
	InheritedFromUniqueProcessID uintptr
}

// NTQueryInformationProcess is used to query process information. // #nosec
func NTQueryInformationProcess(hProcess windows.Handle, class uint8) (interface{}, error) {
	const name = "NTQueryInformationProcess"
	var (
		infoPtr unsafe.Pointer
		size    uintptr
		info    interface{}
	)
	switch class {
	case InfoClassProcessBasicInformation:
		var pbi ProcessBasicInformation
		infoPtr = unsafe.Pointer(&pbi)
		size = unsafe.Sizeof(pbi)
		info = &pbi
	case InfoClassProcessDebugPort,
		InfoClassProcessWow64Information,
		InfoClassProcessImageFileName,
		InfoClassProcessBreakOnTermination,
		InfoClassProcessSubsystemInformation:
		return nil, newError(name, nil, "not implemented")
	default:
		return nil, newErrorf(name, nil, "invalid class: %d", class)
	}
	var returnLength uint32
	ret, _, err := procNTQueryInformationProcess.Call(
		uintptr(hProcess), uintptr(class), uintptr(infoPtr), size,
		uintptr(unsafe.Pointer(&returnLength)),
	)
	if ret != windows.NO_ERROR {
		errno := err.(windows.Errno)
		if errno == windows.ERROR_INSUFFICIENT_BUFFER {
			return returnLength, errno
		}
		return 0, newError(name, errno, "failed to query process information")
	}
	return info, nil
}

// CreateThread is used to create a thread to execute within the
// virtual address space of the calling process. // #nosec
func CreateThread(
	attr *windows.SecurityAttributes, stackSize uint,
	startAddress uintptr, parameters *byte, creationFlags uint32,
) (windows.Handle, uint32, error) {
	const name = "CreateThread"
	var threadID uint32
	ret, _, err := procCreateThread.Call(
		uintptr(unsafe.Pointer(attr)), uintptr(stackSize),
		startAddress, uintptr(unsafe.Pointer(&parameters)), uintptr(creationFlags),
		uintptr(unsafe.Pointer(&threadID)),
	)
	if ret == 0 {
		return 0, 0, newError(name, err, "failed to create thread")
	}
	return windows.Handle(ret), threadID, nil
}

// CreateRemoteThread is used to create a thread that runs in the
// virtual address space of another process. // #nosec
func CreateRemoteThread(
	hProcess windows.Handle, attr *windows.SecurityAttributes, stackSize uint,
	startAddress uintptr, parameters *byte, creationFlags uint32,
) (windows.Handle, uint32, error) {
	const name = "CreateRemoteThread"
	var threadID uint32
	ret, _, err := procCreateRemoteThread.Call(
		uintptr(hProcess), uintptr(unsafe.Pointer(attr)), uintptr(stackSize),
		startAddress, uintptr(unsafe.Pointer(parameters)), uintptr(creationFlags),
		uintptr(unsafe.Pointer(&threadID)),
	)
	if ret == 0 {
		return 0, 0, newError(name, err, "failed to create remote thread")
	}
	return windows.Handle(ret), threadID, nil
}

// ZwCreateThreadEx is used to create remote thread for bypass session isolation.
// in x86 creationFlags can only be 0 "false" and 1 "true". // #nosec
func ZwCreateThreadEx(
	hProcess windows.Handle, attr *windows.SecurityAttributes, stackSize uint,
	startAddress uintptr, parameters *byte, creationFlags uint32,
) (windows.Handle, error) {
	const name = "ZwCreateThreadEx"
	var hThread windows.Handle
	ret, _, err := procZwCreateThreadEx.Call(
		uintptr(unsafe.Pointer(&hThread)), ProcessAllAccess,
		uintptr(unsafe.Pointer(attr)), uintptr(hProcess),
		startAddress, uintptr(unsafe.Pointer(parameters)),
		uintptr(creationFlags), 0, uintptr(stackSize), 0, 0,
	)
	if ret != 0 {
		return 0, newError(name, err, "failed to create remote thread")
	}
	return hThread, nil
}

// reference:
// https://docs.microsoft.com/en-us/windows/win32/api/winternl/ns-winternl-peb

// PEB is the process environment block that contains process information.
type PEB struct {
	InheritedAddressSpace    bool
	ReadImageFileExecOptions bool
	BeingDebugged            bool
	Spare                    bool
	Mutant                   uintptr
	ImageBaseAddress         uintptr
	LoaderData               uintptr // point to PEBLDRData
	ProcessParameters        uintptr
	SubSystemData            uintptr
	ProcessHeap              uintptr
	FastPEBLock              uintptr
	FastPEBLockRoutine       uintptr
	FastPEBUnlockRoutine     uintptr
	// ...
}

// ListEntry include front and back link.
type ListEntry struct {
	FLink uintptr
	BLink uintptr
}

// PEBLDRData contains information about the loaded modules for the process.
type PEBLDRData struct {
	Length                            uint32
	Initialized                       bool
	SsHandle                          uintptr
	InLoadOrderModuleVector           ListEntry
	InMemoryOrderModuleVector         ListEntry
	InInitializationOrderModuleVector ListEntry
}

// LDRDataTableEntry is the loader data table entry.
type LDRDataTableEntry struct {
	InLoadOrderLinks           ListEntry
	InMemoryOrderLinks         ListEntry
	InInitializationOrderLinks ListEntry
	DLLBase                    uintptr
	EntryPoint                 uintptr
	SizeOfImage                uint32
	FullDLLName                LSAUnicodeString
	BaseDLLName                LSAUnicodeString
	// ...
}

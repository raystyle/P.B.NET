// +build windows

package api

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modNTDLL    = windows.NewLazySystemDLL("ntdll.dll")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procNTQueryInformationProcess = modNTDLL.NewProc("NtQueryInformationProcess")
	procReadProcessMemory         = modKernel32.NewProc("ReadProcessMemory")
)

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
	defer func() { _ = windows.Close(snapshot) }()
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
			if err.(syscall.Errno) == windows.ERROR_NO_MORE_FILES {
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
		return nil, newErrorf(name, nil, "process %q is not found", n)
	}
	return pid, nil
}

// OpenProcess is used to open process by PID and return process handle.
func OpenProcess(desiredAccess uint32, inheritHandle bool, pid uint32) (windows.Handle, error) {
	const name = "OpenProcess"
	handle, err := windows.OpenProcess(desiredAccess, inheritHandle, pid)
	if err != nil {
		return 0, newErrorf(name, err, "failed to open process with PID %d", pid)
	}
	return handle, nil
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
func NTQueryInformationProcess(handle windows.Handle, class uint8, info *byte, size uintptr) (uint32, error) {
	const name = "NTQueryInformationProcess"
	var returnLength uint32
	ret, _, err := procNTQueryInformationProcess.Call(
		uintptr(handle), uintptr(class), uintptr(unsafe.Pointer(info)),
		size, uintptr(unsafe.Pointer(&returnLength)),
	)
	if ret != windows.NO_ERROR {
		err := err.(windows.Errno)
		if err == windows.ERROR_INSUFFICIENT_BUFFER {
			return returnLength, err
		}
		return 0, newError(name, err, "failed to query process information")
	}
	return returnLength, nil
}

// ReadProcessMemory is used to read memory from process. // #nosec
func ReadProcessMemory(handle windows.Handle, address uintptr, buffer *byte, size uintptr) (int, error) {
	const name = "ReadProcessMemory"
	var n uint
	ret, _, err := procReadProcessMemory.Call(
		uintptr(handle), address,
		uintptr(unsafe.Pointer(buffer)), size,
		uintptr(unsafe.Pointer(&n)),
	)
	if ret != 1 {
		return 0, newErrorf(name, err, "failed to read process memory at 0x%X", address)
	}
	return int(n), nil
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
	Flink *ListEntry
	Blink *ListEntry
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

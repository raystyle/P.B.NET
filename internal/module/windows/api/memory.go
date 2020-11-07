package api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// reference:
// https://docs.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-readprocessmemory
// https://docs.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-writeprocessmemory
// https://docs.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-VirtualAllocEx

// ReadProcessMemory is used to read memory from process. // #nosec
func ReadProcessMemory(handle windows.Handle, address uintptr, buffer *byte, size uintptr) (int, error) {
	const name = "ReadProcessMemory"
	var n uint
	ret, _, err := procReadProcessMemory.Call(
		uintptr(handle), address,
		uintptr(unsafe.Pointer(buffer)), size,
		uintptr(unsafe.Pointer(&n)),
	)
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to read process memory at 0x%X", address)
	}
	return int(n), nil
}

// WriteProcessMemory is used to write data to memory in process. // #nosec
func WriteProcessMemory(handle windows.Handle, address uintptr, data []byte) (int, error) {
	const name = "WriteProcessMemory"
	var n uint
	ret, _, err := procWriteProcessMemory.Call(
		uintptr(handle), address,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		uintptr(unsafe.Pointer(&n)),
	)
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to write process memory at 0x%X", address)
	}
	return int(n), nil
}

// VirtualAllocEx is used to reserves, commits, or changes the state of a region of memory
// within the virtual address space of a specified process. The function initializes the
// memory it allocates to zero.
func VirtualAllocEx(handle windows.Handle, address uintptr, size uintptr, typ, protect uint32) (uintptr, error) {
	const name = "VirtualAllocEx"
	ret, _, err := procWriteProcessMemory.Call(
		uintptr(handle), address, size, uintptr(typ), uintptr(protect),
	)
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to call VirtualAllocEx")
	}
	return ret, nil
}

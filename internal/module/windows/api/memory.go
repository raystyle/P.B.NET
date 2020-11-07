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
	ret, _, err := procVirtualAllocEx.Call(uintptr(handle), address, size, uintptr(typ), uintptr(protect))
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to alloc memory to remote process")
	}
	return ret, nil
}

// VirtualFreeEx is used to releases, decommits, or releases and decommits a region of memory
// within the virtual address space of a specified process.
func VirtualFreeEx(handle windows.Handle, address uintptr, size uintptr, typ uint32) error {
	const name = "VirtualFreeEx"
	ret, _, err := procVirtualFreeEx.Call(uintptr(handle), address, size, uintptr(typ))
	if ret == 0 {
		return newErrorf(name, err, "failed to free memory about remote process")
	}
	return nil
}

// VirtualProtectEx is used to changes the protection on a region of committed pages in the
// virtual address space of a specified process. // #nosec
func VirtualProtectEx(handle windows.Handle, address uintptr, size uintptr, new uint32, old *uint32) error {
	const name = "VirtualProtectEx"
	ret, _, err := procVirtualProtectEx.Call(
		uintptr(handle), address, size, uintptr(new), uintptr(unsafe.Pointer(old)),
	)
	if ret == 0 {
		return newErrorf(name, err, "failed to change committed pages")
	}
	return nil
}

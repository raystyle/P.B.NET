package api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// reference:
// https://docs.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-readprocessmemory
// https://docs.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-writeprocessmemory

// ReadProcessMemory is used to read memory from process. // #nosec
func ReadProcessMemory(hProcess windows.Handle, addr uintptr, buf *byte, size uintptr) (int, error) {
	const name = "ReadProcessMemory"
	var n uint
	ret, _, err := procReadProcessMemory.Call(
		uintptr(hProcess), addr,
		uintptr(unsafe.Pointer(buf)), size,
		uintptr(unsafe.Pointer(&n)),
	)
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to read process memory at 0x%X", addr)
	}
	return int(n), nil
}

// WriteProcessMemory is used to write data to memory in process. // #nosec
func WriteProcessMemory(hProcess windows.Handle, addr uintptr, data []byte) (int, error) {
	const name = "WriteProcessMemory"
	var n uint
	ret, _, err := procWriteProcessMemory.Call(
		uintptr(hProcess), addr,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		uintptr(unsafe.Pointer(&n)),
	)
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to write process memory at 0x%X", addr)
	}
	return int(n), nil
}

// VirtualAlloc is used to reserves, commits, or changes the state of a region of pages
// in the virtual address space of the calling process. Memory allocated by this function
// is automatically initialized to zero.
func VirtualAlloc(addr, size uintptr, typ, protect uint32) (uintptr, error) {
	const name = "VirtualAlloc"
	ret, _, err := procVirtualAlloc.Call(addr, size, uintptr(typ), uintptr(protect))
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to alloc memory at 0x%X", addr)
	}
	return ret, nil
}

// VirtualAllocEx is used to reserves, commits, or changes the state of a region of memory
// within the virtual address space of a specified process. The function initializes the
// memory it allocates to zero.
func VirtualAllocEx(hProcess windows.Handle, addr, size uintptr, typ, protect uint32) (uintptr, error) {
	const name = "VirtualAllocEx"
	ret, _, err := procVirtualAllocEx.Call(uintptr(hProcess), addr, size, uintptr(typ), uintptr(protect))
	if ret == 0 {
		return 0, newErrorf(name, err, "failed to alloc memory to remote process at 0x%X", addr)
	}
	return ret, nil
}

// VirtualFree is used to releases, decommits, or releases and decommits a region of pages
// within the virtual address space of the calling process.
func VirtualFree(addr, size uintptr, typ uint32) error {
	const name = "VirtualFree"
	ret, _, err := procVirtualFree.Call(addr, size, uintptr(typ))
	if ret == 0 {
		return newErrorf(name, err, "failed to free memory at 0x%X", addr)
	}
	return nil
}

// VirtualFreeEx is used to releases, decommits, or releases and decommits a region of memory
// within the virtual address space of a specified process.
func VirtualFreeEx(hProcess windows.Handle, addr, size uintptr, typ uint32) error {
	const name = "VirtualFreeEx"
	ret, _, err := procVirtualFreeEx.Call(uintptr(hProcess), addr, size, uintptr(typ))
	if ret == 0 {
		return newErrorf(name, err, "failed to free memory about remote process at 0x%X", addr)
	}
	return nil
}

// VirtualProtect is used to change the protection on a region of committed pages in the
// virtual address space of the calling process. // #nosec
func VirtualProtect(addr, size uintptr, new uint32, old *uint32) error {
	const name = "VirtualProtect"
	ret, _, err := procVirtualProtect.Call(
		addr, size, uintptr(new), uintptr(unsafe.Pointer(old)),
	)
	if ret == 0 {
		return newErrorf(name, err, "failed to change committed pages at 0x%X", addr)
	}
	return nil
}

// VirtualProtectEx is used to changes the protection on a region of committed pages in the
// virtual address space of a specified process. // #nosec
func VirtualProtectEx(hProcess windows.Handle, addr, size uintptr, new uint32, old *uint32) error {
	const name = "VirtualProtectEx"
	ret, _, err := procVirtualProtectEx.Call(
		uintptr(hProcess), addr, size, uintptr(new), uintptr(unsafe.Pointer(old)),
	)
	if ret == 0 {
		return newErrorf(name, err, "failed to change committed pages about remote process at 0x%X", addr)
	}
	return nil
}

// VirtualLock is used to lock the specified region of the process's virtual address space into
// physical memory, ensuring that subsequent access to the region will not incur a page fault.
func VirtualLock(addr, size uintptr) error {
	const name = "VirtualLock"
	ret, _, err := procVirtualLock.Call(addr, size)
	if ret == 0 {
		return newErrorf(name, err, "failed to lock page at 0x%X", addr)
	}
	return nil
}

// VirtualUnlock is used to unlock the specified range of pages in the virtual address space of
// a process, enabling the system to swap the pages out to the paging file if necessary.
func VirtualUnlock(addr, size uintptr) error {
	const name = "VirtualUnlock"
	ret, _, err := procVirtualUnlock.Call(addr, size)
	if ret == 0 {
		return newErrorf(name, err, "failed to unlock page at 0x%X", addr)
	}
	return nil
}

// MemoryBasicInformation contains a range of pages in the virtual address space of a process.
// The VirtualQuery and VirtualQueryEx functions use this structure.
type MemoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

// VirtualQuery is used to retrieve information about a range of pages in the virtual address
// space of the calling process. To retrieve information about a range of pages in the address
// space of another process, use the VirtualQueryEx function.
func VirtualQuery(addr uintptr) (*MemoryBasicInformation, error) {
	const name = "VirtualQuery"
	var mbi MemoryBasicInformation
	ret, _, err := procVirtualQuery.Call(addr, uintptr(unsafe.Pointer(&mbi)), unsafe.Sizeof(mbi))
	if ret == 0 {
		return nil, newErrorf(name, err, "failed to query memory information at 0x%X", addr)
	}
	return &mbi, nil
}

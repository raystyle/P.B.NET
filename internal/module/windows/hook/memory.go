// +build windows

package hook

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
)

// #nosec
func unsafeReadMemory(addr uintptr, size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = *(*byte)(unsafe.Pointer(addr + uintptr(i)))
	}
	return data
}

// #nosec
func unsafeWriteMemory(addr uintptr, data []byte) {
	for i := 0; i < len(data); i++ {
		*(*byte)(unsafe.Pointer(addr + uintptr(i))) = data[i]
	}
}

type memory struct {
	Addr       uintptr
	Size       int
	oldProtect *uint32
}

func newMemory(addr uintptr, size int) *memory {
	return &memory{
		Addr:       addr,
		Size:       size,
		oldProtect: new(uint32),
	}
}

func (mem *memory) Write(data []byte) (err error) {
	size := uintptr(len(data))
	err = api.VirtualProtect(mem.Addr, size, windows.PAGE_READWRITE, mem.oldProtect)
	if err != nil {
		return
	}
	defer func() {
		err = api.VirtualProtect(mem.Addr, size, *mem.oldProtect, mem.oldProtect)
	}()
	unsafeWriteMemory(mem.Addr, data)
	return nil
}

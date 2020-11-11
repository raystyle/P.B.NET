// +build windows

package hook

import (
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
)

func unsafeReadMemory(addr uintptr, size int) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("read at invalid memory address")
		}
	}()
	data = make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = *(*byte)(unsafe.Pointer(addr + uintptr(i))) // #nosec
	}
	return
}

func unsafeWriteMemory(addr uintptr, data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("read at invalid memory address")
		}
	}()
	for i := 0; i < len(data); i++ {
		*(*byte)(unsafe.Pointer(addr + uintptr(i))) = data[i] // #nosec
	}
	return
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
		e := api.VirtualProtect(mem.Addr, size, *mem.oldProtect, mem.oldProtect)
		if e != nil && err == nil {
			err = e
		}
	}()
	return unsafeWriteMemory(mem.Addr, data)
}

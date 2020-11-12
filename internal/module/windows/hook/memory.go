// +build windows

package hook

import (
	"golang.org/x/sys/windows"

	"project/internal/module/windows/api"
)

func readMemory(addr uintptr, size int) ([]byte, error) {
	data := make([]byte, size)
	_, err := api.ReadProcessMemory(windows.CurrentProcess(), addr, &data[0], uintptr(size))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func writeMemory(addr uintptr, data []byte) error {
	_, err := api.WriteProcessMemory(windows.CurrentProcess(), addr, data)
	return err
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
	return writeMemory(mem.Addr, data)
}

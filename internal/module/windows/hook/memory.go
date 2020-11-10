// +build windows

package hook

import (
	"unsafe"
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
	Size       uint
	oldProtect uintptr
}

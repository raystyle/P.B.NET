// +build windows

package hook

import (
	"unsafe"
)

type arch interface {
	DisassembleMode() int
	FarJumpSize() int
	NewFarJumpASM(from, to uintptr) []byte
}

func newNearJumpASM(from, to uintptr) []byte {
	const (
		asmOpNearJmp = 0xE9 // jmp rel32
		size         = 1 + 4
	)
	asm := make([]byte, size)
	asm[0] = asmOpNearJmp
	*(*int32)(unsafe.Pointer(&asm[1])) = int32(to) - int32(from) - size
	return asm
}

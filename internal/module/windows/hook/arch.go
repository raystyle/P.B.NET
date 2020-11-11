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

const nearJumperSize = 1 + 4

func newNearJumpASM(from, to uintptr) []byte {
	const asmOpNearJmp = 0xE9 // jmp rel32
	asm := make([]byte, nearJumperSize)
	asm[0] = asmOpNearJmp
	*(*int32)(unsafe.Pointer(&asm[1])) = int32(to) - int32(from) - nearJumperSize
	return asm
}

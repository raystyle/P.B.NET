// +build windows

package hook

import (
	"encoding/binary"
)

type arch386 struct{}

func newArch() arch {
	return arch386{}
}

func (arch386) DisassembleMode() int {
	return 32
}

func (arch386) FarJumpSize() int {
	return 2 + 4 + 4
}

func (a arch386) NewFarJumpASM(from, to uintptr) []byte {
	const asmOpFarJmp = 0x25FF // jmp dword ptr[addr32]
	asm := make([]byte, a.FarJumpSize())
	binary.LittleEndian.PutUint16(asm, asmOpFarJmp)
	binary.LittleEndian.PutUint32(asm[2:], uint32(from+6))
	binary.LittleEndian.PutUint32(asm[6:], uint32(to))
	return asm
}

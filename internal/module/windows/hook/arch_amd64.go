// +build windows

package hook

import (
	"encoding/binary"
)

type archAMD64 struct{}

func newArch() arch {
	return archAMD64{}
}

func (archAMD64) DisassembleMode() int {
	return 64
}

func (archAMD64) FarJumpSize() int {
	return 1 + 4 + 4 + 4 + 1
}

func (a archAMD64) NewFarJumpASM(_, to uintptr) []byte {
	const (
		asmOPPush    = 0x68       // push
		asmOpMovRsp4 = 0x042444C7 // mov DWORD PTR [rsp+0x4], ...
		asmOpRet     = 0xC3       // ret
	)
	asm := make([]byte, a.FarJumpSize())
	asm[0] = asmOPPush
	binary.LittleEndian.PutUint32(asm[1:], lowDword(uint64(to)))
	binary.LittleEndian.PutUint32(asm[5:], asmOpMovRsp4)
	binary.LittleEndian.PutUint32(asm[9:], highDword(uint64(to)))
	asm[13] = asmOpRet
	return asm
}

func lowDword(qWord uint64) uint32 {
	return uint32(qWord & 0xffffffff)
}

func highDword(qWord uint64) uint32 {
	return uint32(qWord >> 32)
}

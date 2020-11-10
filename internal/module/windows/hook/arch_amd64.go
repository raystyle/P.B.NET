// +build windows

package hook

import (
	"encoding/binary"
)

type archAMD64 struct{}

func newArch() (arch, error) {
	return archAMD64{}, nil
}

func (archAMD64) DisassembleMode() int {
	return 64
}

func (archAMD64) JumperSize() int {
	return 14
}

func (archAMD64) NewJumpAsm(_, to uintptr) []byte {
	const (
		opPush    = 0x68       // push
		opMovRsp4 = 0x042444C7 // mov DWORD PTR [rsp+0x4], ...
		opRet     = 0xC3       // ret
	)
	asm := make([]byte, 14)
	asm[0] = opPush
	binary.LittleEndian.PutUint32(asm[1:], lowDword(uint64(to)))
	binary.LittleEndian.PutUint32(asm[5:], opMovRsp4)
	binary.LittleEndian.PutUint32(asm[9:], highDword(uint64(to)))
	asm[13] = opRet
	return asm
}

func lowDword(qWord uint64) uint32 {
	return uint32(qWord & 0xffffffff)
}

func highDword(qWord uint64) uint32 {
	return uint32(qWord >> 32)
}

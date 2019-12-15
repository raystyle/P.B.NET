package light

import (
	"bytes"
	"errors"
	"io"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/rand"
	"project/internal/random"
)

const (
	paddingHeaderSize = 2   // uint16 2 bytes
	paddingMinSize    = 272 // min random padding
	paddingMaxSize    = 512 // max random padding
	passwordSize      = 256 // light crypto
)

// errors
var (
	ErrInvalidPaddingSize  = errors.New("invalid padding size")
	ErrInvalidPasswordSize = errors.New("invalid password size")
)

// client generate curve25519 private data
// send curve25519 public data with padding data
// +--------------+--------------+------------+
// | padding size | padding data | curve25519 |
// +--------------+--------------+------------+
// |    uint16    |     xxx      |     32     |
// +--------------+--------------+------------+
func (c *Conn) clientHandshake() error {
	pri := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, pri)
	pub, err := curve25519.ScalarBaseMult(pri)
	if err != nil {
		return err
	}
	r := random.New()
	sendPaddingSize := paddingMinSize + r.Int(paddingMaxSize)
	handshake := bytes.Buffer{}
	// write padding size
	handshake.Write(convert.Uint16ToBytes(uint16(sendPaddingSize)))
	// write padding data
	handshake.Write(r.Bytes(sendPaddingSize))
	// write curve25519 out
	handshake.Write(pub)
	_, err = c.Conn.Write(handshake.Bytes())
	if err != nil {
		return err
	}
	// receive padding size
	buffer := make([]byte, paddingHeaderSize)
	_, err = io.ReadFull(c.Conn, buffer)
	if err != nil {
		return err
	}
	recvPaddingSize := convert.BytesToUint16(buffer)
	// check padding size
	if recvPaddingSize < paddingMinSize { // <exploit>
		return ErrInvalidPaddingSize
	}
	// receive padding data
	buffer = make([]byte, recvPaddingSize)
	_, err = io.ReadFull(c.Conn, buffer)
	if err != nil {
		return err
	}
	// receive server curve25519 out
	_, err = io.ReadFull(c.Conn, buffer[:32])
	if err != nil {
		return err
	}
	// calculate AES key
	aesKey, err := curve25519.ScalarMult(pri, buffer[:32])
	if err != nil {
		return err
	}
	// receive encrypted password
	_, err = io.ReadFull(c.Conn, buffer[:256+16])
	if err != nil {
		return err
	}
	password, err := aes.CBCDecrypt(buffer[:256+16], aesKey, aesKey[:aes.IVSize])
	if err != nil {
		return err
	}
	if len(password) != passwordSize {
		return ErrInvalidPasswordSize
	}
	c.crypto = newCrypto(password)
	return nil
}

// server generate curve25519 private data,
// after key exchange, generate light password
// use AES CBC to encrypt password,
// send encrypted data with padding data
// +--------------+--------------+------------+----------+
// | padding size | padding data | curve25519 | password |
// +--------------+--------------+------------+----------+
// |    uint16    |      xxx     |     32     |  256+16  |
// +--------------+--------------+------------+----------+
func (c *Conn) serverHandshake() error {
	// receive padding size
	buffer := make([]byte, paddingHeaderSize)
	_, err := io.ReadFull(c.Conn, buffer)
	if err != nil {
		return err
	}
	recvPaddingSize := convert.BytesToUint16(buffer)
	// check padding size
	if recvPaddingSize < paddingMinSize { // <exploit>
		return ErrInvalidPaddingSize
	}
	// receive padding data
	buffer = make([]byte, recvPaddingSize)
	_, err = io.ReadFull(c.Conn, buffer)
	if err != nil {
		return err
	}
	// receive client curve25519 public key
	_, err = io.ReadFull(c.Conn, buffer[:32])
	if err != nil {
		return err
	}
	pri := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, pri)
	pub, err := curve25519.ScalarBaseMult(pri)
	if err != nil {
		return err
	}
	aesKey, err := curve25519.ScalarMult(pri, buffer[:32])
	if err != nil {
		return err
	}
	c.crypto = newCrypto(nil)
	// encrypt password
	password, err := aes.CBCEncrypt(c.crypto[0][:], aesKey, aesKey[:aes.IVSize])
	if err != nil {
		return err
	}
	r := random.New()
	sendPaddingSize := paddingMinSize + r.Int(paddingMaxSize)
	handshake := bytes.Buffer{}
	// write padding size
	handshake.Write(convert.Uint16ToBytes(uint16(sendPaddingSize)))
	// write padding data
	handshake.Write(r.Bytes(sendPaddingSize))
	// write curve25519 public
	handshake.Write(pub)
	// write encrypted password
	handshake.Write(password)
	_, err = c.Conn.Write(handshake.Bytes())
	return err
}

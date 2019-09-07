package light

import (
	"bytes"
	"errors"
	"io"

	"project/internal/convert"
	"project/internal/crypto/rsa"
	"project/internal/random"
)

const (
	paddingHeaderSize = 2    // uint16 2 bytes
	paddingMinSize    = 281  // min padding = rsaPublicKeySize
	paddingMaxSize    = 1024 // max random padding
	rsaBits           = 2136 // need to encrypt 256 bytes data
	rsaPublicKeySize  = 281  // export public key = 281 bytes
	rsaCipherdataSize = 267  // encrypted password = 267 bytes
	passwordSize      = 256  // light
)

var (
	ErrInvalidPaddingSize  = errors.New("invalid padding size")
	ErrInvalidPasswordSize = errors.New("invalid password size")
)

// client generate rsa private key
// send rsa public key with padding data
// +--------------+--------------+----------------+
// | padding size | padding data | rsa public key |
// +--------------+--------------+----------------+
// |      2       |     xxxx     |       281      |
// +--------------+--------------+----------------+
func (c *Conn) clientHandshake() error {
	privateKey, _ := rsa.GenerateKey(rsaBits)
	rand := random.New(0)
	sendPaddingSize := paddingMinSize + rand.Int(paddingMaxSize)
	handshake := bytes.Buffer{}
	// write padding size
	handshake.Write(convert.Uint16ToBytes(uint16(sendPaddingSize)))
	// write padding data
	handshake.Write(rand.Bytes(sendPaddingSize))
	// write rsa public key
	handshake.Write(rsa.ExportPublicKey(&privateKey.PublicKey))
	_, err := c.Conn.Write(handshake.Bytes())
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
	// receive encrypted password
	buffer = buffer[:rsaCipherdataSize]
	_, err = io.ReadFull(c.Conn, buffer)
	if err != nil {
		return err
	}
	password, err := rsa.Decrypt(privateKey, buffer)
	if err != nil {
		return err
	}
	if len(password) != passwordSize {
		return ErrInvalidPasswordSize
	}
	c.crypto = newCrypto(password)
	return nil
}

// server generate light password,
// use rsa public key encrypt it,
// send encrypted data with padding data,
// +--------------+--------------+--------------+
// | padding size | padding data |   password   |
// +--------------+--------------+--------------+
// |      2       |     xxxx     |      267     |
// +--------------+--------------+--------------+
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
	// receive rsa public key
	buffer = buffer[:rsaPublicKeySize]
	_, err = io.ReadFull(c.Conn, buffer)
	if err != nil {
		return err
	}
	publicKey, err := rsa.ImportPublicKey(buffer)
	if err != nil {
		return err
	}
	c.crypto = newCrypto(nil)
	// encrypt password
	cipherdata, err := rsa.Encrypt(publicKey, c.crypto[0][:])
	if err != nil {
		return err
	}
	rand := random.New(0)
	sendPaddingSize := paddingMinSize + rand.Int(paddingMaxSize)
	handshake := bytes.Buffer{}
	// write padding size
	handshake.Write(convert.Uint16ToBytes(uint16(sendPaddingSize)))
	// write padding data
	handshake.Write(rand.Bytes(sendPaddingSize))
	// write encrypted password
	handshake.Write(cipherdata)
	_, err = c.Conn.Write(handshake.Bytes())
	return err
}

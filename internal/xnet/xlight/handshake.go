package xlight

import (
	"bytes"
	"errors"
	"io"

	"project/internal/convert"
	"project/internal/crypto/rsa"
	"project/internal/random"
)

const (
	padding_size        = 2    // uint16 2 bytes
	padding_min_size    = 281  // min padding
	padding_max_size    = 1024 // max random padding
	rsa_bits            = 2136 // need to encrypt 256 bytes data
	rsa_publickey_size  = 281  // export public key = 281 bytes
	rsa_cipherdata_size = 267  // encrypted password = 267 bytes
	password_size       = 256  // light password
)

// client generate rsa private key
// send rsa public key with padding data
// +--------------+--------------+----------------+
// | padding size | padding data | rsa public key |
// +--------------+--------------+----------------+
// |      2       |     xxxx     |       281      |
// +--------------+--------------+----------------+
func (this *Conn) client_handshake() error {
	privatekey, _ := rsa.Generate_Key(rsa_bits)
	generator := random.New()
	padding_size := padding_min_size + generator.Int(padding_max_size)
	handshake := bytes.Buffer{}
	// write padding size
	handshake.Write(convert.Uint16_Bytes(uint16(padding_size)))
	// write padding data
	handshake.Write(generator.Bytes(padding_size))
	// write rsa public key
	handshake.Write(rsa.Export_PublicKey(&privatekey.PublicKey))
	_, err := this.Conn.Write(handshake.Bytes())
	if err != nil {
		return err
	}
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
func (this *Conn) server_handshake() error {
	// stage 2
	// receive padding size
	buffer := make([]byte, padding_size)
	_, err := io.ReadFull(this.Conn, buffer)
	if err != nil {
		return err
	}
	padding_size := convert.Bytes_Uint16(buffer)
	// check padding size
	if padding_size < padding_min_size { // <exploit>
		return errors.New("invalid padding size")
	}
	// receive padding data
	buffer = make([]byte, padding_size)
	_, err = io.ReadFull(this.Conn, buffer)
	if err != nil {
		return err
	}
	// receive rsa public key
	_, err = io.ReadFull(this.Conn, buffer[:rsa_publickey_size])
	if err != nil {
		return err
	}
	publickey, err := rsa.Import_PublicKey(buffer[:rsa_publickey_size])
	if err != nil {
		return err
	}
	rsa.Encrypt(publickey, nil)
	return nil
}

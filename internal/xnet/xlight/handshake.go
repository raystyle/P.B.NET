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
	padding_header_size = 2    // uint16 2 bytes
	padding_min_size    = 281  // min padding
	padding_max_size    = 1024 // max random padding
	rsa_bits            = 2136 // need to encrypt 256 bytes data
	rsa_publickey_size  = 281  // export public key = 281 bytes
	rsa_cipherdata_size = 267  // encrypted password = 267 bytes
	password_size       = 256
)

var (
	ERR_INVALID_PADDING_SIZE  = errors.New("invalid padding size")
	ERR_INVALID_PASSWORD_SIZE = errors.New("invalid password size")
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
	send_padding_size := padding_min_size + generator.Int(padding_max_size)
	handshake := bytes.Buffer{}
	// write padding size
	handshake.Write(convert.Uint16_Bytes(uint16(send_padding_size)))
	// write padding data
	handshake.Write(generator.Bytes(send_padding_size))
	// write rsa public key
	handshake.Write(rsa.Export_PublicKey(&privatekey.PublicKey))
	_, err := this.Conn.Write(handshake.Bytes())
	if err != nil {
		return err
	}
	// receive padding size
	buffer := make([]byte, padding_header_size)
	_, err = io.ReadFull(this.Conn, buffer)
	if err != nil {
		return err
	}
	recv_padding_size := convert.Bytes_Uint16(buffer)
	// check padding size
	if recv_padding_size < padding_min_size { // <exploit>
		return ERR_INVALID_PADDING_SIZE
	}
	// receive padding data
	buffer = make([]byte, recv_padding_size)
	_, err = io.ReadFull(this.Conn, buffer)
	if err != nil {
		return err
	}
	// receive encrypted password
	_, err = io.ReadFull(this.Conn, buffer[:rsa_cipherdata_size])
	if err != nil {
		return err
	}
	password, err := rsa.Decrypt(privatekey, buffer[:rsa_cipherdata_size])
	if err != nil {
		return err
	}
	if len(password) != password_size {
		return ERR_INVALID_PASSWORD_SIZE
	}
	this.cryptor = new_cryptor(password)
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
	// receive padding size
	buffer := make([]byte, padding_header_size)
	_, err := io.ReadFull(this.Conn, buffer)
	if err != nil {
		return err
	}
	recv_padding_size := convert.Bytes_Uint16(buffer)
	// check padding size
	if recv_padding_size < padding_min_size { // <exploit>
		return ERR_INVALID_PADDING_SIZE
	}
	// receive padding data
	buffer = make([]byte, recv_padding_size)
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
	this.cryptor = new_cryptor(nil)
	// encrypt password
	cipherdata, err := rsa.Encrypt(publickey, this.cryptor[0][:])
	if err != nil {
		return err
	}
	generator := random.New()
	send_padding_size := padding_min_size + generator.Int(padding_max_size)
	handshake := bytes.Buffer{}
	// write padding size
	handshake.Write(convert.Uint16_Bytes(uint16(send_padding_size)))
	// write padding data
	handshake.Write(generator.Bytes(send_padding_size))
	// write encrypted password
	handshake.Write(cipherdata)
	_, err = this.Conn.Write(handshake.Bytes())
	return err
}

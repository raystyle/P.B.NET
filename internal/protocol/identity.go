package protocol

import (
	"bytes"
	"errors"

	"project/internal/convert"
	"project/internal/guid"
)

const (
	V1 uint32 = 0
)

type Role uint8

const (
	CONTROLLER Role = iota
	NODE
	BEACON
	AGENT
)

var (
	ERROR_INVALID_ROLE      = errors.New("invalid role")
	ERROR_INVALID_CERT_SIZE = errors.New("invalid certificate size")
	ERROR_INVALID_CERT      = errors.New("invalid certificate")
	ERROR_AUTH_FAILED       = errors.New("authorization failed")
)

var (
	CONTROLLER_GUID       = bytes.Repeat([]byte{0}, guid.SIZE)
	AUTHORIZATION_SUCCESS = []byte("success")
)

type Identity struct {
	Version uint32
	Role    Role
}

func (this *Identity) Encode() []byte {
	return append(convert.Uint32_Bytes(this.Version), byte(this.Role))
}

func (this *Identity) Decode(data []byte) error {
	if len(data) != 5 {
		return errors.New("invalid identity info size")
	}
	this.Version = convert.Bytes_Uint32(data[:4])
	this.Role = Role(data[4])
	return nil
}

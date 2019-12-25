package protocol

import (
	"bytes"
	"errors"
	"fmt"

	"project/internal/guid"
)

// errors about identity
var (
	ErrInvalidCertificate = errors.New("invalid certificate")
	ErrAuthenticateFailed = errors.New("failed to authenticate")
)

// identity
var (
	CtrlGUID    = bytes.Repeat([]byte{0}, guid.Size)
	AuthSucceed = []byte("success")
)

// Role is used to show identity
type Role uint8

// roles
const (
	Ctrl Role = iota
	Node
	Beacon
)

func (role Role) String() string {
	switch role {
	case Ctrl:
		return "controller"
	case Node:
		return "node"
	case Beacon:
		return "beacon"
	default:
		return fmt.Sprintf("invalid role: %d", role)
	}
}

func (role Role) Error() string {
	return role.String()
}

var (
	bytesCtrl   = []byte{0}
	bytesNode   = []byte{1}
	bytesBeacon = []byte{2}
)

// Bytes is used to return bytes
func (role Role) Bytes() []byte {
	switch role {
	case Ctrl:
		return bytesCtrl
	case Node:
		return bytesNode
	case Beacon:
		return bytesBeacon
	default:
		return []byte{255}
	}
}

const (
	byteCtrl = iota
	byteNode
	byteBeacon
)

// Byte is used to return byte
func (role Role) Byte() byte {
	switch role {
	case Ctrl:
		return byteCtrl
	case Node:
		return byteNode
	case Beacon:
		return byteBeacon
	default:
		return 255
	}
}

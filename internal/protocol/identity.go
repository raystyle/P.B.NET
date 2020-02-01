package protocol

import (
	"errors"
	"fmt"

	"project/internal/guid"
)

// ErrAuthenticateFailed is used to client handshake
var ErrAuthenticateFailed = errors.New("failed to authenticate")

// AuthSucceed is used to reply client
var AuthSucceed = []byte{1}

// CtrlGUID is the Controller GUID, it used to reserve
var CtrlGUID = new(guid.GUID)

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

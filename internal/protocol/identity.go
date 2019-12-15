package protocol

import (
	"bytes"
	"errors"
	"fmt"

	"project/internal/guid"
)

// Role is used to show identity
type Role uint8

const (
	// Ctrl is controller, broadcast messages to Nodes,
	// send messages to Nodes or Beacons, and receive
	// broadcast messages or messages sent from Nodes
	// or Beacons.
	Ctrl Role = iota

	// Node broadcast and send messages to controller,
	// and receive broadcast messages or messages sent
	// from controller.
	// store messages sent from controller, nodes and
	// beacons.
	// synchronize message between Nodes.
	Node

	// Beacon broadcast and send messages to controller,
	// and receive messages sent from controller.
	Beacon
)

const (
	byteCtrl = iota
	byteNode
	byteBeacon
)

var (
	bytesCtrl   = []byte{0}
	bytesNode   = []byte{1}
	bytesBeacon = []byte{2}
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

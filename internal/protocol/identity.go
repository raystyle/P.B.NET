package protocol

import (
	"bytes"
	"errors"

	"project/internal/guid"
)

type Role uint8

func (role Role) String() string {
	switch role {
	case Ctrl:
		return "controller"
	case Node:
		return "node"
	case Beacon:
		return "beacon"
	default:
		return "invalid role"
	}
}

func (role Role) Bytes() []byte {
	return []byte{byte(role)}
}

func (role Role) Byte() byte {
	return byte(role)
}

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

var (
	ErrInvalidRole = errors.New("invalid role")
	ErrInvalidCert = errors.New("invalid certificate")
	ErrAuthFailed  = errors.New("authenticate failed")
)

var (
	CtrlGUID    = bytes.Repeat([]byte{0}, guid.Size)
	AuthSucceed = []byte("success")
)

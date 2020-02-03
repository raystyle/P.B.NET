package protocol

import (
	"errors"
	"fmt"

	"project/internal/guid"
)

// Role is used to show identity
type Role byte

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

// Bytes is used to return bytes
func (role Role) Bytes() []byte {
	switch role {
	case Ctrl:
		return []byte{byte(Ctrl)}
	case Node:
		return []byte{byte(Node)}
	case Beacon:
		return []byte{byte(Beacon)}
	default:
		return []byte{255}
	}
}

// CtrlGUID is the Controller GUID, it used to reserve
var CtrlGUID = new(guid.GUID)

// ErrAuthenticateFailed is used to client handshake
var ErrAuthenticateFailed = errors.New("failed to authenticate")

// AuthSucceed is used to reply client
var AuthSucceed = []byte{1}

// about Node operations
const (
	NodeOperationRegister byte = iota + 1
	NodeOperationConnect
)

// about Beacon operations
const (
	BeaconOperationRegister byte = iota + 1
	BeaconOperationConnect
)

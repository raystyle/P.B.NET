package protocol

import (
	"errors"
	"fmt"
	"time"

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

// about Node operations
const (
	NodeOperationRegister byte = iota + 1
	NodeOperationConnect
	NodeOperationUpdate
)

// about Beacon operations
const (
	BeaconOperationRegister byte = iota + 10
	BeaconOperationConnect
	BeaconOperationUpdate
)

var (
	// CtrlGUID is the Controller GUID, it used to reserve
	CtrlGUID = new(guid.GUID)

	// AuthSucceed is used to reply client
	AuthSucceed = []byte{1}

	// ErrAuthenticateFailed is used to client handshake
	ErrAuthenticateFailed = errors.New("failed to authenticate")
)

// NodeKey contains public key, key exchange public key.
type NodeKey struct {
	PublicKey    []byte
	KexPublicKey []byte
	ReplyTime    time.Time
}

// BeaconKey contains public key, key exchange public key.
type BeaconKey struct {
	PublicKey    []byte
	KexPublicKey []byte
	ReplyTime    time.Time
}

// KeyStorage contains all role's key.
type KeyStorage struct {
	NodeKeys   map[guid.GUID]*NodeKey
	BeaconKeys map[guid.GUID]*BeaconKey
}

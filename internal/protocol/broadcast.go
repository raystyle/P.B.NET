package protocol

import (
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
)

// broadcast first send message token,
// if don't handled, send total message.
// token = Role + GUID

var (
	BroadcastUnhandled = []byte{0}
	BroadcastHandled   = []byte{1}
	BroadcastSucceed   = []byte{2}

	ErrBroadcastHandled = errors.New("this broadcast has been handled")
)

// Broadcast is used to broadcast messages to role
// it will not be saved in node database.
//
// When SenderRole = Node or Beacon, Ctrl will handle it.
// SenderRole use their session key to encrypt Message.
//
// When SenderRole = Ctrl, Node will handle it, SenderGUID = CtrlGUID
// Ctrl use broadcast key to encrypt Message.
// look controller/keygen.go GenerateCtrlKeys()
//
// Signature = SenderRole.Sign(GUID + Message + SenderRole + SenderGUID)
type Broadcast struct {
	GUID       []byte // prevent duplicate handle it
	Message    []byte // encrypted
	SenderRole Role
	SenderGUID []byte
	Signature  []byte
}

func (b *Broadcast) Validate() error {
	if len(b.GUID) != guid.Size {
		return errors.New("invalid GUID size")
	}
	if len(b.Message) < aes.BlockSize {
		return errors.New("invalid message size")
	}
	if b.SenderRole > Beacon {
		return errors.New("invalid sender role")
	}
	if len(b.SenderGUID) != guid.Size {
		return errors.New("invalid sender GUID size")
	}
	if len(b.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// BroadcastResponse is use to get broadcast response.
// Role is the receiver role that sender connect it
// if one node connect controller and a node.
// When node send to controller, Role is Ctrl.
// When controller send to node, Role is Node.
type BroadcastResponse struct {
	Role Role
	GUID []byte // Role GUID
	Err  error
}

// BroadcastResponse is use to get broadcast result.
// it include all BroadcastResponse.
type BroadcastResult struct {
	Success  int
	Response []*BroadcastResponse
	Err      error
}

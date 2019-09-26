package protocol

import (
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
	"project/internal/guid"
)

var (
	BroadcastUnhandled = []byte{0}
	BroadcastHandled   = []byte{1}
	BroadcastSucceed   = []byte{2}

	ErrBroadcastHandled = errors.New("this broadcast has been handled")
)

// Broadcast is used to broadcast messages to Nodes.
// Controller use broadcast key to encrypt Message.
// look controller/keygen.go GenerateCtrlKeys()
// Signature = CTRL.global.Sign(GUID + Message + Hash)
type Broadcast struct {
	GUID      []byte // prevent duplicate handle it
	Message   []byte // encrypted
	Hash      []byte // raw message hash
	Signature []byte
}

func (b *Broadcast) Validate() error {
	if len(b.GUID) != guid.Size {
		return errors.New("invalid GUID size")
	}
	if len(b.Message) < aes.BlockSize {
		return errors.New("invalid message size")
	}
	if len(b.Hash) != sha256.Size {
		return errors.New("invalid message hash size")
	}
	if len(b.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// BroadcastResponse is use to get broadcast response.
type BroadcastResponse struct {
	GUID []byte // Node GUID
	Err  error
}

// BroadcastResult is use to get broadcast result.
// it include all BroadcastResponse.
type BroadcastResult struct {
	Success   int
	Responses []*BroadcastResponse
	Err       error
}

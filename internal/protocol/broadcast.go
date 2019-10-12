package protocol

import (
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
	"project/internal/guid"
)

var (
	BroadcastGUIDTimeout = []byte{0}
	BroadcastUnhandled   = []byte{1}
	BroadcastHandled     = []byte{2}
	BroadcastSucceed     = []byte{3}

	ErrBroadcastGUIDTimeout = errors.New("this broadcast GUID is timeout")
	ErrBroadcastHandled     = errors.New("this broadcast has been handled")
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
		return errors.New("invalid hash size")
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

func (br *BroadcastResult) Clean() {
	br.Success = 0
	br.Responses = nil
	br.Err = nil
}

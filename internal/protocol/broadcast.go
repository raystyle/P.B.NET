package protocol

import (
	"crypto/sha256"
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"

	"project/internal/guid"
)

// Broadcast is used to broadcast messages to Nodes.
// Controller use broadcast key to encrypt Message.
// Signature = CTRL.global.Sign(GUID + Message + Hash)
type Broadcast struct {
	GUID      []byte // prevent duplicate handle it
	Message   []byte // encrypted
	Hash      []byte // raw message hash
	Signature []byte
}

// Validate is used to validate broadcast fields
func (b *Broadcast) Validate() error {
	if len(b.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	l := len(b.Message)
	if l < aes.BlockSize || l%aes.BlockSize != 0 {
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

// Clean is used to clean BroadcastResult for sync.Pool
func (br *BroadcastResult) Clean() {
	br.Success = 0
	br.Responses = nil
	br.Err = nil
}

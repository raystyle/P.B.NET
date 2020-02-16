package protocol

import (
	"bytes"
	"crypto/sha256"
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
)

// +----------+----------+-----------+---------+
// |   GUID   |   hash   | signature | message |
// +----------+----------+-----------+---------+
// | 48 bytes | 32 bytes |  64 bytes |   var   |
// +----------+----------+-----------+---------+

// Broadcast is used to broadcast messages to all Nodes,
// Controller use broadcast key to encrypt message
type Broadcast struct {
	GUID      guid.GUID // prevent duplicate handle it
	Hash      []byte    // raw message hash
	Signature []byte    // sign(GUID + Hash + Message)
	Message   []byte    // use gzip and AES to compress and encrypt
}

// NewBroadcast is used to create a broadcast, Unpack() need it
// if only used to Pack(), use new(Broadcast).
func NewBroadcast() *Broadcast {
	return &Broadcast{
		Hash:      make([]byte, sha256.Size),
		Signature: make([]byte, ed25519.SignatureSize),
		Message:   make([]byte, 2*aes.BlockSize),
	}
}

// Pack is used to pack Broadcast to *bytes.Buffer
func (b *Broadcast) Pack(buf *bytes.Buffer) {
	buf.Write(b.GUID[:])
	buf.Write(b.Hash)
	buf.Write(b.Signature)
	buf.Write(b.Message)
}

// Unpack is used to unpack []byte to Broadcast
func (b *Broadcast) Unpack(data []byte) error {
	if len(data) < guid.Size+sha256.Size+ed25519.SignatureSize+aes.BlockSize {
		return errors.New("invalid broadcast packet size")
	}
	copy(b.GUID[:], data[:guid.Size])
	copy(b.Hash, data[guid.Size:guid.Size+sha256.Size])
	copy(b.Signature, data[guid.Size+sha256.Size:guid.Size+sha256.Size+ed25519.SignatureSize])
	message := data[guid.Size+sha256.Size+ed25519.SignatureSize:]
	mLen := len(message)
	bmLen := len(b.Message)
	if cap(b.Message) >= mLen {
		switch {
		case bmLen > mLen:
			copy(b.Message, message)
			b.Message = b.Message[:mLen]
		case bmLen == mLen:
			copy(b.Message, message)
		case bmLen < mLen:
			b.Message = append(b.Message[:0], message...)
		}
	} else {
		b.Message = make([]byte, mLen)
		copy(b.Message, message)
	}
	return nil
}

// Validate is used to validate broadcast fields
func (b *Broadcast) Validate() error {
	if len(b.Hash) != sha256.Size {
		return errors.New("invalid hash size")
	}
	if len(b.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	l := len(b.Message)
	if l < aes.BlockSize || l%aes.BlockSize != 0 {
		return errors.New("invalid message size")
	}
	return nil
}

// BroadcastResponse is use to get broadcast response.
type BroadcastResponse struct {
	GUID *guid.GUID // Node GUID
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

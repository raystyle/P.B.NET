package protocol

import (
	"errors"

	"project/internal/guid"
)

// broadcast first send message token
// if don't handled send total message
// token = role + guid

var (
	BroadcastUnhandled = []byte{0}
	BroadcastHandled   = []byte{1}
	BroadcastSucceed   = []byte{2}

	ErrBroadcastHandled = errors.New("this broadcast handled")
)

// broadcast message to role
// worker
type Broadcast struct {
	GUID         []byte
	Message      []byte // AES encrypted
	SenderRole   Role
	SenderGUID   []byte
	ReceiverRole Role
	Signature    []byte
}

func (b *Broadcast) Validate() error {
	if len(b.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	if len(b.Message) < 16 {
		return errors.New("invalid message")
	}
	if b.SenderRole > Beacon {
		return errors.New("invalid sender role")
	}
	if len(b.SenderGUID) != guid.SIZE {
		return errors.New("invalid sender guid")
	}
	if b.Signature == nil {
		return errors.New("no signature")
	}
	if b.SenderRole == b.ReceiverRole {
		return errors.New("same sender receiver role")
	}
	return nil
}

type BroadcastResponse struct {
	Role Role
	GUID []byte
	Err  error
}

type BroadcastResult struct {
	Success  int
	Response []*BroadcastResponse
	Err      error
}

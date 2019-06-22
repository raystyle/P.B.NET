package protocol

import (
	"errors"

	"project/internal/guid"
)

//broadcast first send message token
//if don't handled send total message
//token = role + guid

var (
	BROADCAST_UNHANDLED = []byte{0}
	BROADCAST_HANDLED   = []byte{1}
	BROADCAST_SUCCESS   = []byte{2}

	ERROR_BROADCAST_HANDLED = errors.New("this broadcast handled")
)

//broadcast message to role
//worker
type Broadcast struct {
	GUID          []byte
	Message       []byte //AES
	Sender_Role   Role
	Sender_GUID   []byte
	Receiver_Role Role
	Signature     []byte //ECDSA(total)
}

func (this *Broadcast) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	if len(this.Message) < 16 {
		return errors.New("invalid message")
	}
	if this.Sender_Role > BEACON {
		return errors.New("invalid sender role")
	}
	if len(this.Sender_GUID) != guid.SIZE {
		return errors.New("invalid sender guid")
	}
	if this.Signature == nil {
		return errors.New("invalid signature")
	}
	if this.Sender_Role == this.Receiver_Role {
		return errors.New("same sender&receiver role")
	}
	return nil
}

type Broadcast_Response struct {
	Role Role
	GUID []byte
	Err  error
}

type Broadcast_Result struct {
	Success  int
	Response []*Broadcast_Response
	Err      error
}

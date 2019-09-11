package protocol

import (
	"errors"

	"project/internal/guid"
)

//sync_x first send message token
//if don't handled send total message
//token = role + guid

var (
	SYNC_UNHANDLED = []byte{0}
	SYNC_HANDLED   = []byte{1}
	SYNC_SUCCESS   = []byte{2}

	ERROR_SYNC_HANDLED = errors.New("this sync handled")
	ERROR_NO_NODES     = errors.New("no connected nodes")
	ERROR_NO_MESSAGE   = errors.New("no message")
)

//----------------------------send message---------------------------------
//worker
type Sync_Send struct {
	GUID          []byte
	Height        uint64
	Message       []byte //AES
	Sender_Role   Role
	Sender_GUID   []byte
	Receiver_Role Role
	Receiver_GUID []byte
	Signature     []byte //ECDSA(total)
}

func (this *Sync_Send) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	if len(this.Message) < 16 {
		return errors.New("invalid message")
	}
	if this.Sender_Role > Beacon {
		return errors.New("invalid sender role")
	}
	if len(this.Sender_GUID) != guid.SIZE {
		return errors.New("invalid sender guid")
	}
	if this.Receiver_Role > Beacon {
		return errors.New("invalid receiver role")
	}
	if len(this.Receiver_GUID) != guid.SIZE {
		return errors.New("invalid receiver guid")
	}
	if this.Signature == nil {
		return errors.New("invalid signature")
	}
	if this.Sender_Role == this.Receiver_Role {
		return errors.New("same sender&receiver role")
	}
	return nil
}

type SyncReceive struct {
	GUID          []byte
	Height        uint64
	Receiver_Role Role
	Receiver_GUID []byte
	Signature     []byte //ECDSA(total)
}

func (this *SyncReceive) Validate() error {
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	if this.Receiver_Role != Beacon && this.Receiver_Role != Node {
		return errors.New("invalid receiver role")
	}
	if len(this.Receiver_GUID) != guid.SIZE {
		return errors.New("invalid receiver guid")
	}
	if this.Signature == nil {
		return errors.New("invalid signature")
	}
	return nil
}

type SyncResponse struct {
	Role Role
	GUID []byte
	Err  error
}

type SyncResult struct {
	Success  int
	Response []*SyncResponse
	Err      error
}

// -------------------------active sync message-----------------------------

type SyncQuery struct {
	Role   Role
	GUID   []byte
	Height uint64
}

func (this *SyncQuery) Validate() error {
	if this.Role != Beacon && this.Role != Node {
		return errors.New("invalid role")
	}
	if len(this.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	return nil
}

type SyncReply struct {
	GUID      []byte //sync_send.GUID
	Message   []byte //sync_send.Message
	Signature []byte //sync_send.Signature
	Err       error
}

// new message > 2 || search lastest message
type Sync_Task struct {
	Role Role
	GUID []byte
}

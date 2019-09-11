package protocol

import (
	"errors"

	"project/internal/guid"
)

// syncXXXX first send message token
// if don't handled send total message
// token = role + guid

var (
	SyncUnhandled = []byte{0}
	SyncHandled   = []byte{1}
	SyncSucceed   = []byte{2}

	ErrSyncHandled = errors.New("this sync handled")
	ErrNoNodes     = errors.New("no connected nodes")
	ErrNoMessage   = errors.New("no message")
)

// ----------------------------send message---------------------------------
// worker
type SyncSend struct {
	GUID         []byte
	Height       uint64
	Message      []byte // encrypted
	SenderRole   Role
	SenderGUID   []byte
	ReceiverRole Role
	ReceiverGUID []byte
	Signature    []byte
}

func (ss *SyncSend) Validate() error {
	if len(ss.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	if len(ss.Message) < 16 {
		return errors.New("invalid message")
	}
	if ss.SenderRole > Beacon {
		return errors.New("invalid sender role")
	}
	if len(ss.SenderGUID) != guid.SIZE {
		return errors.New("invalid sender guid")
	}
	if ss.ReceiverRole > Beacon {
		return errors.New("invalid receiver role")
	}
	if len(ss.ReceiverGUID) != guid.SIZE {
		return errors.New("invalid receiver guid")
	}
	if ss.Signature == nil {
		return errors.New("invalid signature")
	}
	if ss.SenderRole == ss.ReceiverRole {
		return errors.New("same sender&receiver role")
	}
	return nil
}

type SyncReceive struct {
	GUID         []byte
	Height       uint64
	ReceiverRole Role
	ReceiverGUID []byte
	Signature    []byte
}

func (sr *SyncReceive) Validate() error {
	if len(sr.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	if sr.ReceiverRole != Beacon && sr.ReceiverRole != Node {
		return errors.New("invalid receiver role")
	}
	if len(sr.ReceiverGUID) != guid.SIZE {
		return errors.New("invalid receiver guid")
	}
	if sr.Signature == nil {
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

func (sq *SyncQuery) Validate() error {
	if sq.Role != Beacon && sq.Role != Node {
		return errors.New("invalid role")
	}
	if len(sq.GUID) != guid.SIZE {
		return errors.New("invalid guid")
	}
	return nil
}

type SyncReply struct {
	GUID      []byte // syncSend.GUID
	Message   []byte // syncSend.Message
	Signature []byte // syncSend.Signature
	Err       error
}

// new message > 2 || search latest message
type SyncTask struct {
	Role Role
	GUID []byte
}

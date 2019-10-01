package protocol

import (
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
	"project/internal/guid"
)

var (
	SendUnhandled = []byte{3}
	SendHandled   = []byte{4}
	SendSucceed   = []byte{5}

	ErrSendHandled = errors.New("this send has been handled")
)

// -------------------------------interactive mode----------------------------------

// Send is used to send messages in interactive mode.
//
// When Controller use it, Role and RoleGUID = receiver
// role and receiver GUID. Controller encrypt Message
// with Node or Beacon session key.
// When Node use it, Role = Node and RoleGUID = its GUID.
// Node encrypt Message with its session key.
// When Beacon use it, Role = Beacon and RoleGUID = its GUID.
// Beacon encrypt Message with its session key.
//
// Signature = role.global.Sign(GUID + Message + Hash + Role + RoleGUID)
type Send struct {
	GUID      []byte // prevent duplicate handle it
	Message   []byte // encrypted
	Hash      []byte // raw message hash
	Role      Role
	RoleGUID  []byte
	Signature []byte
}

func (s *Send) Validate() error {
	if len(s.GUID) != guid.Size {
		return errors.New("invalid GUID size")
	}
	if len(s.Message) < aes.BlockSize {
		return errors.New("invalid message size")
	}
	if len(s.Hash) != sha256.Size {
		return errors.New("invalid message hash size")
	}
	if s.Role > Beacon {
		return errors.New("invalid role")
	}
	if len(s.RoleGUID) != guid.Size {
		return errors.New("invalid role GUID size")
	}
	if len(s.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

type SendResponse struct {
	Role Role
	GUID []byte // Role GUID
	Err  error
}

type SendResult struct {
	Reply     []byte // Role Reply
	Success   int
	Responses []*SendResponse
	Err       error
}

func (sr *SendResult) Clean() {
	sr.Reply = nil
	sr.Success = 0
	sr.Responses = nil
	sr.Err = nil
}

// Acknowledge is used to acknowledge sender that receiver
// has receive this message
//
// When Controller use it, Role and RoleGUID = sender role
// and sender GUID.
// When Node use it, Role = Node and RoleGUID = its GUID.
// When Beacon use it, Role = Beacon and RoleGUID = its GUID.
//
// Signature = role.global.Sign(GUID + Role + RoleGUID + SendGUID)
type Acknowledge struct {
	GUID      []byte // prevent duplicate handle it
	Role      Role
	RoleGUID  []byte
	SendGUID  []byte // Send.GUID
	Signature []byte
}

func (ack *Acknowledge) Validate() error {
	if len(ack.GUID) != guid.Size {
		return errors.New("invalid GUID size")
	}
	if ack.Role > Beacon {
		return errors.New("invalid role")
	}
	if len(ack.RoleGUID) != guid.Size {
		return errors.New("invalid role GUID size")
	}
	if len(ack.SendGUID) != guid.Size {
		return errors.New("invalid role GUID size")
	}
	if len(ack.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// -------------------------------query mode----------------------------------

// SyncReceive is used to synchronize node_receive,
// beacon_receive, controller_receive, (look database tables)
// all roles will use it.
//
// When Ctrl send message to Node or Beacon, and they receive it,
// they will send SyncReceive to they connected Nodes,
// Node will delete corresponding controller send message.
//
// When Node or Beacon send message to Ctrl, and Ctrl receive it,
// Ctrl will send SyncReceive to they connected Nodes,
// Node will delete corresponding role send message.
//
// Signature = SenderRole.Sign(GUID + Height + Role + RoleGUID)
type SyncReceive struct {
	GUID      []byte // prevent duplicate handle it
	Height    uint64
	Role      Role
	RoleGUID  []byte
	Signature []byte
}

func (srr *SyncReceive) Validate() error {
	if len(srr.GUID) != guid.Size {
		return errors.New("invalid GUID size")
	}
	if srr.Role > Beacon {
		return errors.New("invalid role")
	}
	if len(srr.RoleGUID) != guid.Size {
		return errors.New("invalid role GUID size")
	}
	if len(srr.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// ---------------------active synchronize message----------------------

// SyncQuery is used to query message from role.
// When Ctrl use it, it can query messages sent to Ctrl by roles.
// When Node use it, it can query all messages,
// Ctrl <-> Node, Ctrl <-> Beacon
// When Beacon use it, it can query messages sent to Beacon by Ctrl.
type SyncQuery struct {
	GUID  []byte
	Index uint64 // message index
}

func (sq *SyncQuery) Validate() error {
	if len(sq.GUID) != guid.Size {
		return errors.New("invalid GUID Size")
	}
	return nil
}

// SyncReply is the reply of SyncQuery.
type SyncReply struct {
	GUID      []byte // SyncSend.GUID
	Message   []byte // SyncSend.Message
	Hash      []byte // SyncSend.Hash
	Signature []byte // SyncSend.Signature
	Err       error
}

func (sr *SyncReply) Validate() error {
	if sr.Err == nil {
		if len(sr.GUID) != guid.Size {
			return errors.New("invalid GUID Size")
		}
		if len(sr.Message) < aes.BlockSize {
			return errors.New("invalid message size")
		}
		if len(sr.Hash) != sha256.Size {
			return errors.New("invalid message hash size")
		}
		if len(sr.Signature) != ed25519.SignatureSize {
			return errors.New("invalid signature size")
		}
	}
	return nil
}

// SyncTask is used to tell syncer.worker to
// synchronize message actively.
type SyncTask struct {
	Role Role
	GUID []byte
}

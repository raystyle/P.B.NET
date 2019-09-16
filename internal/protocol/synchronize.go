package protocol

import (
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
)

// syncXXXX first send message token
// if don't handled send total message
// token = role + guid

var (
	SyncUnhandled = []byte{3}
	SyncHandled   = []byte{4}
	SyncSucceed   = []byte{5}

	ErrSyncHandled     = errors.New("this sync has been handled")
	ErrNoSyncerClients = errors.New("no connected syncer client")
	ErrNotExistMessage = errors.New("this message is not exist")
	ErrWorkerStopped   = errors.New("worker stopped")
)

// ----------------------------send message---------------------------------

// SyncSend is used to send messages to role
// it will be saved in node database.
//
// SenderRole can be Ctrl, Node, Beacon
// When SenderRole = Ctrl, SenderGUID = CtrlGUID,
// Ctrl use their session key to encrypt Message.
// When SenderRole = Node or Beacon, ReceiverRole = Ctrl,
// SenderRole use their session key to encrypt Message.
// look Role/global.go
//
// Signature = SenderRole.Sign(GUID + Height + Message +
// SenderRole + SenderGUID + ReceiverRole + ReceiverGUID)
type SyncSend struct {
	GUID         []byte // prevent duplicate handle it
	Height       uint64
	Message      []byte // encrypted
	SenderRole   Role
	SenderGUID   []byte
	ReceiverRole Role
	ReceiverGUID []byte
	Signature    []byte
}

func (ss *SyncSend) Validate() error {
	if len(ss.GUID) != guid.Size {
		return errors.New("invalid GUID size")
	}
	if len(ss.Message) < aes.BlockSize {
		return errors.New("invalid message size")
	}
	if ss.SenderRole > Beacon {
		return errors.New("invalid sender role")
	}
	if len(ss.SenderGUID) != guid.Size {
		return errors.New("invalid sender GUID size")
	}
	if ss.ReceiverRole > Beacon {
		return errors.New("invalid receiver role")
	}
	if len(ss.ReceiverGUID) != guid.Size {
		return errors.New("invalid receiver GUID size")
	}
	if len(ss.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	if ss.SenderRole == ss.ReceiverRole {
		return errors.New("sender and receiver are the same")
	}
	return nil
}

// SyncRoleReceive is used to synchronize node_receive,
// beacon_receive, controller_receive, (look database tables)
// all roles will use it.
//
// When Ctrl send message to Node or Beacon, and they receive it,
// they will send SyncRoleReceive to they connected Nodes,
// Node will delete corresponding controller send message.
//
// When Node or Beacon send message to Ctrl, and Ctrl receive it,
// Ctrl will send SyncRoleReceive to they connected Nodes,
// Node will delete corresponding role send message.
//
// Signature = SenderRole.Sign(GUID + Height + Role + RoleGUID)
type SyncRoleReceive struct {
	GUID      []byte // prevent duplicate handle it
	Height    uint64
	Role      Role
	RoleGUID  []byte
	Signature []byte
}

func (srr *SyncRoleReceive) Validate() error {
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

// SyncResponse is use to get synchronize response
// Role is the receiver role that sender connect it
// if one node connect controller and a node.
// When node send to controller, Role is Ctrl.
// When controller send to node, Role is Node.
type SyncResponse struct {
	Role Role
	GUID []byte // Role GUID
	Err  error
}

// SyncResult is use to get synchronize result
// it include all SyncResponse
type SyncResult struct {
	Success  int
	Response []*SyncResponse
	Err      error
}

// ---------------------active synchronize message----------------------

// SyncQuery is used to query message from role
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

// SyncReply is the reply of SyncQuery
type SyncReply struct {
	GUID      []byte // SyncSend.GUID
	Message   []byte // SyncSend.Message
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
		if len(sr.Signature) != ed25519.SignatureSize {
			return errors.New("invalid signature size")
		}
	}
	return nil
}

// SyncTask is used to tell syncer.worker to
// synchronize message actively
type SyncTask struct {
	Role Role
	GUID []byte
}

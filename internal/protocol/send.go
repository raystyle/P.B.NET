package protocol

import (
	"errors"
	"fmt"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
	"project/internal/guid"
)

const SendMinBufferSize = 2*guid.Size + aes.BlockSize + sha256.Size + ed25519.SignatureSize

var (
	SendReplyExpired   = []byte{10}
	SendReplyUnhandled = []byte{11}
	SendReplyHandled   = []byte{12}
	SendReplySucceed   = []byte{13}

	ErrSendExpired = errors.New("send expired")
	ErrSendHandled = errors.New("send has been handled")
)

// GetSendReplyError is used to get get send error from reply
func GetSendReplyError(reply []byte) error {
	if len(reply) == 0 {
		return errors.New("empty send reply")
	}
	switch reply[0] {
	case SendReplyExpired[0]:
		return ErrSendExpired
	case SendReplyHandled[0]:
		return ErrSendHandled
	default:
		return fmt.Errorf("unknown send reply error: %X", reply)
	}
}

// --------------------------interactive mode-------------------------------

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
// Signature = role.global.Sign(GUID + RoleGUID + Message + Hash)
type Send struct {
	GUID      []byte // prevent duplicate handle it
	RoleGUID  []byte // receiver GUID
	Message   []byte // encrypted
	Hash      []byte // raw message hash
	Signature []byte
}

// Validate is used to validate send fields
func (s *Send) Validate() error {
	if len(s.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(s.RoleGUID) != guid.Size {
		return errors.New("invalid role guid size")
	}
	l := len(s.Message)
	if l < aes.BlockSize || l%aes.BlockSize != 0 {
		return errors.New("invalid message size")
	}
	if len(s.Hash) != sha256.Size {
		return errors.New("invalid hash size")
	}
	if len(s.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// SendResponse is use to get send response.
type SendResponse struct {
	Role Role
	GUID []byte // Role GUID
	Err  error
}

// SendResult is use to get send result.
// it include all SendResponse.
type SendResult struct {
	Success   int
	Responses []*SendResponse
	Err       error
}

// Clean is used to clean SendResult for sync.Pool
func (sr *SendResult) Clean() {
	sr.Success = 0
	sr.Responses = nil
	sr.Err = nil
}

// Acknowledge is used to acknowledge sender that receiver
// has receive this message
//
// When Controller use it, RoleGUID = sender GUID.
// When Node use it, RoleGUID = it's GUID.
// When Beacon use it, RoleGUID = it's GUID.
//
// Signature = role.global.Sign(GUID + RoleGUID + SendGUID)
type Acknowledge struct {
	GUID      []byte // prevent duplicate handle it
	RoleGUID  []byte
	SendGUID  []byte // Send.GUID
	Signature []byte
}

// Validate is used to validate acknowledge fields
func (ack *Acknowledge) Validate() error {
	if len(ack.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(ack.RoleGUID) != guid.Size {
		return errors.New("invalid role guid size")
	}
	if len(ack.SendGUID) != guid.Size {
		return errors.New("invalid send guid size")
	}
	if len(ack.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// AcknowledgeResponse is use to get acknowledge response.
type AcknowledgeResponse struct {
	Role Role
	GUID []byte // Role GUID
	Err  error
}

// AcknowledgeResult is use to get acknowledge result.
// it include all AcknowledgeResponse.
type AcknowledgeResult struct {
	Success   int
	Responses []*AcknowledgeResponse
	Err       error
}

// Clean is used to clean AcknowledgeResult for sync.Pool
func (ar *AcknowledgeResult) Clean() {
	ar.Success = 0
	ar.Responses = nil
	ar.Err = nil
}

// -------------------------------query mode--------------------------------

// Query is used to query message from controller,
// only Beacon will use it, because Node always
// in interactive mode
type Query struct {
	GUID       []byte // prevent duplicate handle it
	BeaconGUID []byte
	Index      uint64
	Signature  []byte
}

// Validate is used to validate query fields
func (q *Query) Validate() error {
	if len(q.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(q.BeaconGUID) != guid.Size {
		return errors.New("invalid beacon guid size")
	}
	if len(q.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// QueryResponse is use to get query response.
type QueryResponse struct {
	Role Role
	GUID []byte // Role GUID
	Err  error
}

// QueryResult is use to get query result.
// it include all QueryResponse.
type QueryResult struct {
	Success   int
	Responses []*QueryResponse
	Err       error
}

// Clean is used to clean QueryResult for sync.Pool
func (qr *QueryResult) Clean() {
	qr.Success = 0
	qr.Responses = nil
	qr.Err = nil
}

// Answer is used to return queried message
type Answer struct {
	GUID       []byte // prevent duplicate handle it
	BeaconGUID []byte
	Index      uint64
	Message    []byte // encrypted
	Hash       []byte // raw message hash
	Signature  []byte
}

// Validate is used to validate Answer fields
func (a *Answer) Validate() error {
	if len(a.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(a.BeaconGUID) != guid.Size {
		return errors.New("invalid beacon guid size")
	}
	l := len(a.Message)
	if l < aes.BlockSize || l%aes.BlockSize != 0 {
		return errors.New("invalid message size")
	}
	if len(a.Hash) != sha256.Size {
		return errors.New("invalid hash size")
	}
	if len(a.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	return nil
}

// AnswerResponse is use to get answer response.
type AnswerResponse struct {
	Role Role
	GUID []byte // Role GUID
	Err  error
}

// AnswerResult is use to get answer result.
// it include all AnswerResponse.
type AnswerResult struct {
	Success   int
	Responses []*AnswerResponse
	Err       error
}

// Clean is used to clean AnswerResult for sync.Pool
func (ar *AnswerResult) Clean() {
	ar.Success = 0
	ar.Responses = nil
	ar.Err = nil
}

package protocol

import (
	"bytes"
	"crypto/sha256"
	"errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
)

const (
	// SendMinBufferSize is the sender and worker minimum buffer size
	SendMinBufferSize = 2*guid.Size + aes.BlockSize + sha256.Size + ed25519.SignatureSize

	// AcknowledgeSize is the acknowledge packet size
	AcknowledgeSize = 3*guid.Size + ed25519.SignatureSize
)

// --------------------------interactive mode-------------------------------
// in interactive mode, role send message will use "Send" to send message
// and wait "Acknowledge" to make sure message has reached.
// Node is always in interactive mode
// in default, beacon use query mode, you can switch interactive mode manually.

// +----------+-----------+----------+-----------+---------+
// |   guid   | role guid |   hash   | signature | message |
// +----------+-----------+----------+-----------+---------+
// | 52 bytes |  52 bytes | 32 bytes |  64 bytes |   var   |
// +----------+-----------+----------+-----------+---------+

// Send is used to send messages in interactive mode.
//
// When Controller use it, RoleGUID = receiver role GUID.
// Controller encrypt Message with Node or Beacon session key.
// When Node use it, RoleGUID = its GUID.
// Node encrypt Message with it's session key.
// When Beacon use it, RoleGUID = its GUID.
// Beacon encrypt Message with it's session key.
type Send struct {
	GUID      []byte // prevent duplicate handle it
	RoleGUID  []byte // receiver GUID
	Hash      []byte // raw message hash
	Signature []byte // sign(GUID + RoleGUID + Hash + Message)
	Message   []byte // encrypted
}

// NewSend is used to create a send, Unpack() need it,
// if only used to Pack(), use new(Send).
func NewSend() *Send {
	return &Send{
		GUID:      make([]byte, guid.Size),
		RoleGUID:  make([]byte, guid.Size),
		Hash:      make([]byte, sha256.Size),
		Signature: make([]byte, ed25519.SignatureSize),
		Message:   make([]byte, 2*aes.BlockSize),
	}
}

// Pack is used to pack Send to *bytes.Buffer
func (s *Send) Pack(buf *bytes.Buffer) {
	buf.Write(s.GUID)
	buf.Write(s.RoleGUID)
	buf.Write(s.Hash)
	buf.Write(s.Signature)
	buf.Write(s.Message)
}

// Unpack is used to unpack []byte to Send
func (s *Send) Unpack(data []byte) error {
	if len(data) < 2*guid.Size+sha256.Size+ed25519.SignatureSize+aes.BlockSize {
		return errors.New("invalid send packet size")
	}
	copy(s.GUID, data[:guid.Size])
	copy(s.RoleGUID, data[guid.Size:2*guid.Size])
	copy(s.Hash, data[2*guid.Size:2*guid.Size+sha256.Size])
	copy(s.Signature, data[2*guid.Size+sha256.Size:2*guid.Size+sha256.Size+ed25519.SignatureSize])
	message := data[2*guid.Size+sha256.Size+ed25519.SignatureSize:]
	mLen := len(message)
	smLen := len(s.Message)
	if cap(s.Message) >= mLen {
		switch {
		case smLen > mLen:
			copy(s.Message, message)
			s.Message = s.Message[:mLen]
		case smLen == mLen:
			copy(s.Message, message)
		case smLen < mLen:
			s.Message = s.Message[:0]
			s.Message = append(s.Message, message...)
		}
	} else {
		s.Message = make([]byte, mLen)
		copy(s.Message, message)
	}
	return nil
}

// Validate is used to validate send fields
func (s *Send) Validate() error {
	if len(s.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(s.RoleGUID) != guid.Size {
		return errors.New("invalid role guid size")
	}
	if len(s.Hash) != sha256.Size {
		return errors.New("invalid hash size")
	}
	if len(s.Signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	l := len(s.Message)
	if l < aes.BlockSize || l%aes.BlockSize != 0 {
		return errors.New("invalid message size")
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

// +----------+-----------+-----------+-----------+
// |   guid   | role guid | send guid | signature |
// +----------+-----------+-----------+-----------+
// | 52 bytes |  52 bytes |  52 bytes |  64 bytes |
// +----------+-----------+-----------+-----------+

// Acknowledge is used to acknowledge sender that receiver has receive this message
//
// When Controller use it, RoleGUID = sender GUID.
// When Node use it, RoleGUID = it's GUID.
// When Beacon use it, RoleGUID = it's GUID.
type Acknowledge struct {
	GUID      []byte // prevent duplicate handle it
	RoleGUID  []byte //
	SendGUID  []byte // structure Send.GUID
	Signature []byte // sign(GUID + RoleGUID + SendGUID)
}

// NewAcknowledge is used to create a acknowledge, Unpack() need it,
// if only used to Pack(), use new(Acknowledge).
func NewAcknowledge() *Acknowledge {
	return &Acknowledge{
		GUID:      make([]byte, guid.Size),
		RoleGUID:  make([]byte, guid.Size),
		SendGUID:  make([]byte, guid.Size),
		Signature: make([]byte, ed25519.SignatureSize),
	}
}

// Pack is used to pack Acknowledge to *bytes.Buffer
func (ack *Acknowledge) Pack(buf *bytes.Buffer) {
	buf.Write(ack.GUID)
	buf.Write(ack.RoleGUID)
	buf.Write(ack.SendGUID)
	buf.Write(ack.Signature)
}

// Unpack is used to unpack []byte to Acknowledge
func (ack *Acknowledge) Unpack(data []byte) error {
	if len(data) != AcknowledgeSize {
		return errors.New("invalid acknowledge packet size")
	}
	copy(ack.GUID, data[:guid.Size])
	copy(ack.RoleGUID, data[guid.Size:2*guid.Size])
	copy(ack.SendGUID, data[2*guid.Size:3*guid.Size])
	copy(ack.Signature, data[3*guid.Size:3*guid.Size+ed25519.SignatureSize])
	return nil
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

// Query is used to query message from controller, only Beacon will use it,
// because Node always in interactive mode.
type Query struct {
	GUID       []byte `msgpack:"signature"` // prevent duplicate handle it
	BeaconGUID []byte `msgpack:"un"`
	Index      uint64 `msgpack:"pw"`
	Signature  []byte `msgpack:"note"`
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
//
// <security> use fake structure field tag
type Answer struct {
	GUID       []byte `msgpack:"signature"` // prevent duplicate handle it
	BeaconGUID []byte `msgpack:"token"`
	Index      uint64 `msgpack:"index"`
	Message    []byte `msgpack:"un"` // encrypted
	Hash       []byte `msgpack:"pw"` // raw message hash
	Signature  []byte `msgpack:"note"`
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

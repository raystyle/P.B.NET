package protocol

import (
	"errors"
	"fmt"
)

// errors
var (
	ErrReplyExpired = errors.New("expired")
	ErrReplyHandled = errors.New("operation has been handled")
)

// replies
var (
	ReplyUnhandled = []byte{11}
	ReplySucceed   = []byte{13}
	ReplyExpired   = []byte{10}
	ReplyHandled   = []byte{12}
)

// GetReplyError is used to get error from reply
func GetReplyError(reply []byte) error {
	if len(reply) == 0 {
		return errors.New("empty reply")
	}
	switch reply[0] {
	case ReplyExpired[0]:
		return ErrReplyExpired
	case ReplyHandled[0]:
		return ErrReplyHandled
	default:
		return fmt.Errorf("custom error: %s", reply)
	}
}

// TestCommand is used to test role/client.go
const TestCommand uint8 = 0xFF

// -----------------------Connection---------------------------
const (
	ConnSendHeartbeat uint8 = 0x00 + iota
	ConnReplyHeartbeat
	ConnReply

	ErrCMDRecvNullMsg uint8 = 0x0F
	ErrCMDTooBigMsg   uint8 = 0x0E
)

// -----------------------Controller---------------------------
const (
	CtrlSync uint8 = 0x10 + iota
	CtrlSendToNodeGUID
	CtrlSendToNode
	CtrlAckToNodeGUID
	CtrlAckToNode
	CtrlSendToBeaconGUID
	CtrlSendToBeacon
	CtrlAckToBeaconGUID
	CtrlAckToBeacon
	CtrlBroadcastGUID
	CtrlBroadcast
	CtrlAnswerGUID
	CtrlAnswer
)

// about trust node
const (
	CtrlTrustNode uint8 = 0x20 + iota
	CtrlSetNodeCert
)

// --------------------------Node-----------------------------
const (
	NodeSync uint8 = 0x60 + iota
	NodeSendGUID
	NodeSend
	NodeAckGUID
	NodeAck
)

// -------------------------Beacon-----------------------------
const (
	BeaconSendGUID uint8 = 0xA0 + iota
	BeaconSend
	BeaconAckGUID
	BeaconAck
	BeaconQueryGUID
	BeaconQuery
)

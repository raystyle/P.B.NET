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
	ReplyUnhandled = []byte{10}
	ReplySucceed   = []byte{11}
	ReplyExpired   = []byte{20}
	ReplyHandled   = []byte{21}
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
	ConnReply uint8 = 0x00 + iota
	ConnSendHeartbeat
	ConnReplyHeartbeat
	ConnErrRecvNullFrame
	ConnErrRecvTooBigFrame
	ConnGetAddress
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

// --------------------------Node------------------------------
const (
	NodeSync uint8 = 0x60 + iota
	NodeSendGUID
	NodeSend
	NodeAckGUID
	NodeAck
)

// -------------------------Beacon-----------------------------
const (
	BeaconSync uint8 = 0xA0 + iota
	BeaconSendGUID
	BeaconSend
	BeaconAckGUID
	BeaconAck
	BeaconQueryGUID
	BeaconQuery
)

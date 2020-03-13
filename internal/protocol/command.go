package protocol

import (
	"errors"
	"fmt"
)

// errors about reply
var (
	ErrReplyExpired = errors.New("expired")
	ErrReplyHandled = errors.New("operation has been handled")
)

// about reply
var (
	ReplyUnhandled = []byte{10}
	ReplySucceed   = []byte{11}
	ReplyExpired   = []byte{20}
	ReplyHandled   = []byte{21}
)

// GetReplyError is used to get error from reply.
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

// TestCommand is used to test role/client.go.
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

// before synchronize
const (
	// start synchronize
	CtrlSync uint8 = 0x10 + iota

	// about trust Node
	CtrlTrustNode
	CtrlSetNodeCert
	CtrlGetListenerTag

	// for recovery role session key
	CtrlQueryKeyStorage
)

// after synchronize
const (
	CtrlSendToNodeGUID uint8 = 0x30 + iota
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

// --------------------------Node------------------------------

// before synchronize
const (
	// start synchronize
	NodeSync uint8 = 0x50 + iota
)

// after synchronize
const (
	NodeSendGUID uint8 = 0x70 + iota
	NodeSend
	NodeAckGUID
	NodeAck
)

// -------------------------Beacon-----------------------------

// before synchronize
const (
	// start synchronize
	BeaconSync uint8 = 0xA0 + iota
)

// after synchronize
const (
	BeaconSendGUID uint8 = 0xC0 + iota
	BeaconSend
	BeaconAckGUID
	BeaconAck
	BeaconQueryGUID
	BeaconQuery
)

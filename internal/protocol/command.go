package protocol

import (
	"errors"
	"fmt"
)

var (
	ReplyUnhandled = []byte{11}
	ReplySucceed   = []byte{13}
	ReplyExpired   = []byte{10}
	ReplyHandled   = []byte{12}

	ErrReplyExpired = errors.New("expired")
	ErrReplyHandled = errors.New("operation has been handled")
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

// ----------------------Connection---------------------------

const (
	ConnSendHeartbeat uint8 = 0x00 + iota
	ConnReplyHeartbeat
	ConnReply
)

// -----------------------Controller--------------------------
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

const (
	CtrlTrustNode uint8 = 0x20 + iota
	CtrlSetNodeCert
)

const (
	CtrlQueryNodeStatus uint8 = 0x30 + iota
	CtrlQueryAllNodes
)

// --------------------------Node-----------------------------
const (
	NodeSync uint8 = 0x60 + iota
	NodeSendGUID
	NodeSend
	NodeAckGUID
	NodeAck
)

// node authentication
const (
	NodeQueryCertificate uint8 = 0x70 + iota
)

// query nodes
const (
	NodeQueryGUID uint8 = 0x80 + iota
	NodeQueryAllNodes
)

// --------------------------Beacon-----------------------------
const (
	BeaconSendGUID uint8 = 0xA0 + iota
	BeaconSend
	BeaconAckGUID
	BeaconAck
	BeaconQueryGUID
	BeaconQuery
)

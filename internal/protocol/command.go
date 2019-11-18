package protocol

// --------------------------test-----------------------------
const (
	TestCommand uint8 = 0xEF
)

// ----------------------Connection---------------------------

const (
	ConnSendHeartbeat uint8 = 0x00 + iota
	ConnReplyHeartbeat
	ConnReply
)

// -----------------------Controller--------------------------
const (
	CtrlBroadcastGUID uint8 = 0x10 + iota
	CtrlBroadcast
	CtrlSendToNodeGUID
	CtrlSendToNode
	CtrlSendToBeaconGUID
	CtrlSendToBeacon
	CtrlAckToNodeGUID
	CtrlAckToNode
	CtrlAckToBeaconGUID
	CtrlAckToBeacon
)

const (
	CtrlTrustNode uint8 = 0x20 + iota
	CtrlTrustNodeData
)

const (
	CtrlQueryNodeStatus uint8 = 0x30 + iota
	CtrlQueryAllNodes
)

// --------------------------Node-----------------------------
const (
	NodeSendGUID uint8 = 0x60 + iota
	NodeSend
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
	BeaconQueryGUID
	BeaconQuery
)

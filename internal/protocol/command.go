package protocol

// --------------------------test-----------------------------
const (
	TestCommand uint8 = 0xEF
)

// -----------------------Controller--------------------------
const (
	CtrlHeartbeat uint8 = 0x00 + iota
	CtrlReply

	CtrlSync
	CtrlBroadcastGUID
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
	NodeHeartbeat uint8 = 0x60 + iota
	NodeReply

	NodeSync
	NodeSendGUID
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
	BeaconHeartbeat uint8 = 0xA0 + iota
	BeaconReply

	BeaconSyncStart
	BeaconSendGUID
	BeaconSend
	BeaconQueryGUID
	BeaconQuery
)

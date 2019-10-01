package protocol

// --------------------------test-----------------------------
const (
	TestCommand uint8 = 0xEF
)

// -----------------------controller--------------------------
const (
	CtrlHeartbeat uint8 = 0x00 + iota
	CtrlReply
	CtrlSyncStart
	CtrlBroadcastToken
	CtrlBroadcast
	CtrlSyncSendToken
	CtrlSyncSend
	CtrlSyncReceiveToken
	CtrlSyncReceive
	CtrlSyncQueryNode
	CtrlSyncQueryBeacon
)

// trust node
const (
	CtrlTrustNode uint8 = 0x20 + iota
	CtrlTrustNodeData
)

const (
	CtrlQueryNodeStatus uint8 = 0x30 + iota
	CtrlQueryAllNodes
)

// --------------------------node-----------------------------
const (
	NodeHeartbeat uint8 = 0x00 + iota
	NodeReply
	NodeSyncStart
	NodeSyncSendToken
	NodeSyncSend
	NodeSyncReceiveToken
	NodeSyncReceive
)

// node authentication
const (
	NodeQueryCertificate uint8 = 0x20 + iota
)

// query nodes
const (
	NodeQueryGUID uint8 = 0x30 + iota
	NodeQueryAllNodes
)

package protocol

// --------------------------test-----------------------------
const (
	TEST_MSG uint8 = 0xEF
)

// -----------------------controller--------------------------
const (
	CTRL_HEARTBEAT uint8 = 0x00 + iota
	CTRL_REPLY
	CTRL_SYNC_START
	CTRL_SYNC_QUERY
	CTRL_BROADCAST_TOKEN
	CTRL_BROADCAST
	CTRL_SYNC_SEND_TOKEN
	CTRL_SYNC_SEND
	CTRL_SYNC_RECV_TOKEN
	CTRL_SYNC_RECV
)

// trust node
const (
	CTRL_TRUST_NODE uint8 = 0x20 + iota
	CTRL_TRUST_NODE_DATA
)

const (
	CTRL_QUERY_STATUS uint8 = 0x30 + iota
	CTRL_QUERY_ALL_NODES
)

// --------------------------node-----------------------------
const (
	NODE_HEARTBEAT uint8 = 0x00 + iota
	NODE_REPLY
	NODE_SYNC_START
	NODE_SYNC_QUERY
	NODE_BROADCAST_TOKEN
	NODE_BROADCAST
	NODE_SYNC_SEND_TOKEN
	NODE_SYNC_SEND
	NODE_SYNC_RECV_TOKEN
	NODE_SYNC_RECV
)

// node authentication
const (
	NODE_QUERY_CERTIFICATE uint8 = 0x20 + iota
)

// query nodes
const (
	NODE_QUERY_GUID uint8 = 0x30 + iota
	NODE_QUERY_ALL_NODES
)

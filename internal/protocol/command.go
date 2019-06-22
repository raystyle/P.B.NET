package protocol

const (
	CTRL_HEARTBEAT uint8 = 0x00 + iota
)

//-----------------------Controller--------------------------
const (
	CTRL_REPLY uint8 = 0x10 + iota
	CTRL_BROADCAST_TOKEN
	CTRL_BROADCAST
	CTRL_SYNC_SEND_TOKEN
	CTRL_SYNC_SEND
	CTRL_SYNC_RECEIVE_TOKEN
	CTRL_SYNC_RECEIVE
	CTRL_SYNC_START
	CTRL_SYNC_QUERY
)

//node authentication
const (
	CTRL_QUERY_CERTIFICATE uint8 = 0x20 + iota
	CTRL_TRUST_NODE_REQUEST
	CTRL_TRUST_NODE_DATA
)

//query nodes
const (
	CTRL_QUERY_GUID uint8 = 0x30 + iota
	CTRL_QUERY_STATUS
	CTRL_QUERY_ALL_NODES
)

const (
	NODE_HEARTBEAT uint8 = 0x00 + iota
)

//--------------------------Node-----------------------------
const (
	NODE_REPLY uint8 = 0x10 + iota
	NODE_BROADCAST_TOKEN
	NODE_BROADCAST
	NODE_SYNC_SEND_TOKEN
	NODE_SYNC_SEND
	NODE_SYNC_RECEIVE_TOKEN
	NODE_SYNC_RECEIVE
	NODE_SYNC_START
	NODE_SYNC_QUERY
)

//node authentication
const (
	NODE_QUERY_CERTIFICATE uint8 = 0x20 + iota
)

//query nodes
const (
	NODE_QUERY_GUID uint8 = 0x30 + iota
	NODE_QUERY_ALL_NODES
)

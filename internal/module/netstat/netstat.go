package netstat

import (
	"net"
)

// Refresher is used to get current network status.
type Refresher interface {
	Refresh() ([]*Connection, error)
}

// about connection protocol
const (
	_ uint8 = iota
	ProtocolTCP
	ProtocolUDP
)

// about TCP connection state, reference MIB_TCP_STATE
const (
	_ uint8 = iota
	TCPStateClosed
	TCPStateListen
	TCPStateSYNSent
	TCPStateSYNReceived
	TCPStateEstablished
	TCPStateFinWait1
	TCPStateFinWait2
	TCPStateCloseWait
	TCPStateClosing
	TCPStateLastAck
	TCPStateTimeWait
	TCPStateDeleteTCB
)

// Connection include connection information.
type Connection struct {
	Protocol      uint8
	LocalAddr     net.IP
	LocalScopeID  uint32 // about IPv6
	LocalPort     uint16
	RemoteAddr    net.IP
	RemoteScopeID uint32 // about IPv6
	RemotePort    uint16
	State         uint8
	PID           int64
}

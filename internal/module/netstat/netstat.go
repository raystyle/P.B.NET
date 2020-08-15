package netstat

import (
	"net"
)

// NetStat is used to get current network status.
type NetStat interface {
	GetTCP4Conns() ([]*TCP4Conn, error)
	GetTCP6Conns() ([]*TCP6Conn, error)
	GetUDP4Conns() ([]*UDP4Conn, error)
	GetUDP6Conns() ([]*UDP6Conn, error)
}

// NewNetStat is used to create a netstat.
func NewNetStat() (NetStat, error) {
	return newNetstat()
}

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

// TCP4Conn contains information about TCP Over IPv4 connection.
type TCP4Conn struct {
	LocalAddr  net.IP
	LocalPort  uint16
	RemoteAddr net.IP
	RemotePort uint16
	State      uint8
	PID        int64
}

// TCP6Conn contains information about TCP Over IPv6 connection.
type TCP6Conn struct {
	LocalAddr     net.IP
	LocalScopeID  uint32
	LocalPort     uint16
	RemoteAddr    net.IP
	RemoteScopeID uint32
	RemotePort    uint16
	State         uint8
	PID           int64
}

// UDP4Conn contains information about UDP Over IPv4 connection.
type UDP4Conn struct {
	LocalAddr net.IP
	LocalPort uint16
	PID       int64
}

// UDP6Conn contains information about UDP Over IPv6 connection.
type UDP6Conn struct {
	LocalAddr    net.IP
	LocalScopeID uint32
	LocalPort    uint16
	PID          int64
}

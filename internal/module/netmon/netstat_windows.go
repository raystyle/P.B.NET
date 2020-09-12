// +build windows

package netmon

import (
	"project/internal/module/windows/api"
)

// Options contain options about table class.
type Options struct {
	TCPTableClass uint32
	UDPTableClass uint32
}

type netStat struct {
	tcpTableClass uint32
	udpTableClass uint32
}

// NewNetStat is used to create a netstat with TCP and UDP table class.
func NewNetStat(opts *Options) (NetStat, error) {
	if opts == nil {
		opts = &Options{
			TCPTableClass: api.TCPTableOwnerModuleAll,
			UDPTableClass: api.UDPTableOwnerModule,
		}
	}
	return &netStat{
		tcpTableClass: opts.TCPTableClass,
		udpTableClass: opts.UDPTableClass,
	}, nil
}

func (n *netStat) GetTCP4Conns() ([]*TCP4Conn, error) {
	conns, err := api.GetTCP4Conns(n.tcpTableClass)
	if err != nil {
		return nil, err
	}
	l := len(conns)
	cs := make([]*TCP4Conn, l)
	for i := 0; i < l; i++ {
		cs[i] = &TCP4Conn{
			LocalAddr:  conns[i].LocalAddr,
			LocalPort:  conns[i].LocalPort,
			RemoteAddr: conns[i].RemoteAddr,
			RemotePort: conns[i].RemotePort,
			State:      conns[i].State,
			PID:        conns[i].PID,
			Process:    conns[i].Process,
		}
	}
	return cs, nil
}

func (n *netStat) GetTCP6Conns() ([]*TCP6Conn, error) {
	conns, err := api.GetTCP6Conns(n.tcpTableClass)
	if err != nil {
		return nil, err
	}
	l := len(conns)
	cs := make([]*TCP6Conn, l)
	for i := 0; i < l; i++ {
		cs[i] = &TCP6Conn{
			LocalAddr:     conns[i].LocalAddr,
			LocalScopeID:  conns[i].LocalScopeID,
			LocalPort:     conns[i].LocalPort,
			RemoteAddr:    conns[i].RemoteAddr,
			RemoteScopeID: conns[i].RemoteScopeID,
			RemotePort:    conns[i].RemotePort,
			State:         conns[i].State,
			PID:           conns[i].PID,
			Process:       conns[i].Process,
		}
	}
	return cs, nil
}

func (n *netStat) GetUDP4Conns() ([]*UDP4Conn, error) {
	conns, err := api.GetUDP4Conns(n.udpTableClass)
	if err != nil {
		return nil, err
	}
	l := len(conns)
	cs := make([]*UDP4Conn, l)
	for i := 0; i < l; i++ {
		cs[i] = &UDP4Conn{
			LocalAddr: conns[i].LocalAddr,
			LocalPort: conns[i].LocalPort,
			PID:       conns[i].PID,
			Process:   conns[i].Process,
		}
	}
	return cs, nil
}

func (n *netStat) GetUDP6Conns() ([]*UDP6Conn, error) {
	conns, err := api.GetUDP6Conns(n.udpTableClass)
	if err != nil {
		return nil, err
	}
	l := len(conns)
	cs := make([]*UDP6Conn, l)
	for i := 0; i < l; i++ {
		cs[i] = &UDP6Conn{
			LocalAddr:    conns[i].LocalAddr,
			LocalScopeID: conns[i].LocalScopeID,
			LocalPort:    conns[i].LocalPort,
			PID:          conns[i].PID,
			Process:      conns[i].Process,
		}
	}
	return cs, nil
}

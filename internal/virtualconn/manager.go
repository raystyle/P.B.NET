package virtualconn

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"project/internal/guid"
	"project/internal/random"
)

// uint32
const portSize = 4

// ConnID = self GUID(local address) + port(uint32) +
//          role GUID(remote address) + port(uint32)
type ConnID [guid.Size + portSize + guid.Size + portSize]byte

// NewConnID is used to create a connection id.
func NewConnID(local *guid.GUID, lPort uint32, remote *guid.GUID, rPort uint32) *ConnID {
	cid := ConnID{}
	copy(cid[:guid.Size], local[:])
	binary.BigEndian.PutUint32(cid[guid.Size:guid.Size+portSize], lPort)
	copy(cid[guid.Size+4:], remote[:])
	binary.BigEndian.PutUint32(cid[2*guid.Size+portSize:], rPort)
	return &cid
}

// LocalGUID is used to get local GUID in the connection id.
func (cid *ConnID) LocalGUID() *guid.GUID {
	g := guid.GUID{}
	copy(g[:], cid[:guid.Size])
	return &g
}

// LocalPort is used to get local port in the connection id.
func (cid *ConnID) LocalPort() uint32 {
	return binary.BigEndian.Uint32(cid[guid.Size : guid.Size+portSize])
}

// LocalAddr is used to get local address in the connection id.
func (cid *ConnID) LocalAddr() net.Addr {
	return newVCAddr(cid.LocalGUID(), cid.LocalPort())
}

// State is used to show the virtual connection state.
type State uint8

func (s State) String() string {
	switch s {
	case StateListen:
		return "listen"
	case StateESTABLISHED:
		return "established"
	default:
		return fmt.Sprintf("unknown state: %d", uint8(s))
	}
}

// about connection state
const (
	StateListen State = 1 + iota
	StateESTABLISHED
)

type conn struct {
	io.Closer           // *Conn or *Listener
	state     State     // conn state
	usage     string    // like PID
	lastUsed  time.Time // useless for listener
	rwm       sync.RWMutex
}

type sender struct {
	dstGUID *guid.GUID // only controller need it
	dstPort uint32
	srcPort uint32
}

func (s *sender) Send(ctx context.Context, data []byte) error {
	return nil
}

type receiver struct {
	data chan []byte
}

func (r *receiver) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-r.data:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *receiver) pushData() {

}

// senderFunc is used to call Role.sender.Send().
// data include source and destination port.
// data = src port + dst port + payload
type senderFunc func(ctx context.Context, guid *guid.GUID, data []byte) error

// Manager is used to manage listeners and dial connection,
// do keep-alive between connections.
type Manager struct {
	local  *guid.GUID // self address(role GUID)
	sender senderFunc
	now    func() time.Time

	// for select port
	rand *random.Rand

	// conns include all connections that Listen() and Dial()
	ports map[uint32]struct{}
	conns map[ConnID]*conn
	rwm   sync.RWMutex
}

// NewManager is used to create a virtual connection manager.
func NewManager(local *guid.GUID, sender senderFunc, now func() time.Time) *Manager {
	manager := Manager{
		local:  local,
		sender: sender,
		now:    now,
		rand:   random.New(),
	}
	return &manager
}

// Listen is used to bind and return a Listener and port.
// timeout is used to control Listener.Accept() timeout.
// only the equal the remote guid can dial this Listener.
func (m *Manager) Listen(remote *guid.GUID, timeout time.Duration, usage string) (*Listener, uint32) {
	m.rwm.Lock()
	defer m.rwm.Unlock()

	// select a random and doesn't exist port
	var port uint32
	for {
		port = uint32(1 + m.rand.Int(1<<32-1))
		_, ok := m.ports[port]
		if !ok {
			break
		}
	}
	cid := NewConnID(m.local, port, remote, 0)

	listener := NewListener(m.local, cid.LocalPort(), timeout)
	// add to connection pool.
	m.conns[*cid] = &conn{
		Closer:   listener,
		state:    StateListen,
		usage:    usage,
		lastUsed: m.now(),
	}
	return listener, port
}

// IncomeConn is used to notice manager that some one Dial,
func (m *Manager) IncomeConn(remote *guid.GUID, srcPort, dstPort uint32) bool {

	return false
}

// DataArrival is used to push data to a virtual connection
func (m *Manager) DataArrival() {

}

// Dial is
// last replace the port
func (m *Manager) Dial() {

}

// CloseConn is used to kill connection.
func (m *Manager) CloseConn() {

}

// Close is used to close virtual connection manager.
// It will close all listeners and connections.
func (m *Manager) Close() {

	m.now = nil
}

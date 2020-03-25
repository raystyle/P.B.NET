package virtualconn

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"project/internal/guid"
	"project/internal/random"
)

// ConnID = self GUID(local address) + port(uint32) +
//          role GUID(remote address) + port(uint32)
type ConnID [guid.Size + 4 + guid.Size + 4]byte

// LocalGUID is used
func (cid *ConnID) LocalGUID() {

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

// about state
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
	roleGUID *guid.GUID
	port     uint32
}

func (s *sender) Send(ctx context.Context, data []byte) error {
	return nil
}

type receiver struct {
}

func (r *receiver) Receive(ctx context.Context) ([]byte, error) {
	return nil, nil
}

// Manager is used to manage listeners and dial connection,
// do keep-alive between connections.
type Manager struct {
	now  func() time.Time
	rand *random.Rand

	// conns include all connections that Dial()
	conns map[ConnID]*conn

	rwm sync.RWMutex
}

// NewManager is used to create a virtual connection manager.
func NewManager(now func() time.Time) *Manager {
	manager := Manager{
		now:  now,
		rand: random.New(),
	}
	return &manager
}

// Listen is used to bind and return a Listener and port
func (m *Manager) Listen(local *guid.GUID) (*Listener, uint32) {

	return nil, 0
}

// IncomeConn is used to notice manager that some one Dial,
func (m *Manager) IncomeConn() {

}

// Dial is
func (m *Manager) Dial() {

}

// DataArrival is used to push data to a virtual connection
func (m *Manager) DataArrival() {

}

// Close is used to close virtual connection manager.
// It will close all listeners and connections.
func (m *Manager) Close() {

	m.now = nil
}

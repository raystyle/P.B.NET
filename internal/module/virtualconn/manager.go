package virtualconn

import (
	"sync"

	"project/internal/guid"
)

// self GUID(local address) + port(uint32) + role GUID(remote address) + port(uint32)
type connID [guid.Size + 4 + guid.Size + 4]byte

type conn struct {
}

// Manager is a
type Manager struct {
	localAddr guid.GUID

	list map[connID]struct{}
	// listen is used to accept virtual connection
	listeners map[connID]*Listener

	// conns

	rwm sync.RWMutex
}

// NewManager is used to create a virtual connection manager.
func NewManager() *Manager {
	return nil
}

// IncomeDial is used to notice manager that someone Dial,
func (m *Manager) IncomeDial() {

}

// DataArrival is used to push data to a virtual connection
func (m *Manager) DataArrival() {

}

// Listen is
func (m *Manager) Listen() {

}

// Dial is
func (m *Manager) Dial() {

}

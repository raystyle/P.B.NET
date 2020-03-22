package module

import (
	"sync"
)

// Module is the internal module.
type Module interface {
	Start() error
	Stop()
	Restart() error
	Name() string
	Info() string
	Status() string
}

// Manager is the module manager.
type Manager struct {
	// key = module tag
	modules    map[string]Module
	modulesRWM sync.RWMutex
}

// NewManager is used to create a module manager.
func NewManager() {

}

// Add is used to add module to manager.
func (m *Manager) Add(tag string, module Module) {

}

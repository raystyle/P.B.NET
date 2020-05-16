package module

import (
	"sync"

	"github.com/pkg/errors"
)

// Module is the interface of module, it include internal and external module.
//
// Internal module is in the internal/module/*. These module usually use less
// space (use the exist go packages that in GOROOT/src and go.mod), have high
// stability, don't need external program.
//
// External module is in the project/module, or app/mod. These module usually
// have the client(Beacon) and server(external program), client is used to send
// command to the server and receive the result. Client and server can use Pipe
// and Socket for communication. These module maybe not have high stability and
// execute high risk operation.
// Use Start() to connect the module server, and use Call() to send command.
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
	closed bool
	// key = module tag
	modules    map[string]Module
	modulesRWM sync.RWMutex
}

// NewManager is used to create a module manager.
func NewManager() *Manager {
	return &Manager{
		modules: make(map[string]Module),
	}
}

// Add is used to add a module to manager.
func (m *Manager) Add(tag string, module Module) error {
	if tag == "" {
		return errors.New("empty module tag")
	}
	m.modulesRWM.Lock()
	defer m.modulesRWM.Unlock()
	if m.closed {
		return errors.New("proxy server manager closed")
	}
	if _, ok := m.modules[tag]; !ok {
		m.modules[tag] = module
		return nil
	}
	return errors.Errorf("module %s already exists", tag)
}

// Delete is used to delete a module by tag.
func (m *Manager) Delete(tag string) error {
	if tag == "" {
		return errors.New("empty module tag")
	}
	m.modulesRWM.Lock()
	defer m.modulesRWM.Unlock()
	if module, ok := m.modules[tag]; ok {
		module.Stop()
		delete(m.modules, tag)
		return nil
	}
	return errors.Errorf("module %s doesn't exist", tag)
}

// Get is used to get a module by tag.
func (m *Manager) Get(tag string) (Module, error) {
	if tag == "" {
		return nil, errors.New("empty module tag")
	}
	m.modulesRWM.RLock()
	defer m.modulesRWM.RUnlock()
	if module, ok := m.modules[tag]; ok {
		return module, nil
	}
	return nil, errors.Errorf("module %s doesn't exist", tag)
}

// Start is used to start a module by tag.
func (m *Manager) Start(tag string) error {
	module, err := m.Get(tag)
	if err != nil {
		return err
	}
	return module.Start()
}

// Stop is used to stop a module by tag.
func (m *Manager) Stop(tag string) error {
	module, err := m.Get(tag)
	if err != nil {
		return err
	}
	module.Stop()
	return nil
}

// Restart is used to restart a module by tag.
func (m *Manager) Restart(tag string) error {
	module, err := m.Get(tag)
	if err != nil {
		return err
	}
	return module.Restart()
}

// Info is used to get module information by tag.
func (m *Manager) Info(tag string) (string, error) {
	module, err := m.Get(tag)
	if err != nil {
		return "", err
	}
	return module.Info(), nil
}

// Status is used to get module status by tag.
func (m *Manager) Status(tag string) (string, error) {
	module, err := m.Get(tag)
	if err != nil {
		return "", err
	}
	return module.Status(), nil
}

// Modules is used to get all modules.
func (m *Manager) Modules() map[string]Module {
	m.modulesRWM.RLock()
	defer m.modulesRWM.RUnlock()
	modules := make(map[string]Module, len(m.modules))
	for tag, module := range m.modules {
		modules[tag] = module
	}
	return modules
}

// Close is used to stop all modules.
func (m *Manager) Close() {
	m.modulesRWM.Lock()
	defer m.modulesRWM.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	for tag, module := range m.modules {
		module.Stop()
		delete(m.modules, tag)
	}
}

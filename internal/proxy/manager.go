package proxy

import (
	"sync"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
)

// Manager is a proxy server manager
type Manager struct {
	logger  logger.Logger
	servers map[string]*Server // key = tag
	rwm     sync.RWMutex
}

// NewManager is used to create a proxy server manager
func NewManager(lg logger.Logger) *Manager {
	return &Manager{logger: lg, servers: make(map[string]*Server)}
}

// Add is used to add proxy server, but not listen or serve
func (m *Manager) Add(server *Server) error {
	if server.Tag == "" {
		return errors.New("empty proxy server tag")
	}
	deleteServer := func() {
		m.rwm.Lock()
		delete(m.servers, server.Tag)
		m.rwm.Unlock()
	}
	switch server.Mode {
	case ModeSocks:
		opts := new(socks.Options)
		err := toml.Unmarshal([]byte(server.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
		opts.ExitFunc = deleteServer
		// because the tag is never empty, it will never go wrong
		s, _ := socks.NewServer(server.Tag, m.logger, opts)
		server.server = s
	case ModeHTTP:
		opts := new(http.Options)
		err := toml.Unmarshal([]byte(server.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
		opts.ExitFunc = deleteServer
		s, err := http.NewServer(server.Tag, m.logger, opts)
		if err != nil {
			return err
		}
		server.server = s
	default:
		return errors.Errorf("unknown mode %s", server.Mode)
	}
	server.createAt = time.Now()
	m.rwm.Lock()
	defer m.rwm.Unlock()
	if _, ok := m.servers[server.Tag]; !ok {
		m.servers[server.Tag] = server
		return nil
	} else {
		return errors.Errorf("proxy server %s already exists", server.Tag)
	}
}

// Delete is used to delete proxy server
// it will self delete it
func (m *Manager) Delete(tag string) error {
	if tag == "" {
		return errors.New("empty proxy server tag")
	}
	m.rwm.Lock()
	if server, ok := m.servers[tag]; ok {
		m.rwm.Unlock()
		return server.Close()
	} else {
		m.rwm.Unlock()
		return errors.Errorf("proxy server %s doesn't exist", tag)
	}
}

// Get is used to get proxy server
func (m *Manager) Get(tag string) (*Server, error) {
	if tag == "" {
		return nil, errors.New("empty proxy server tag")
	}
	m.rwm.RLock()
	defer m.rwm.RUnlock()
	if server, ok := m.servers[tag]; ok {
		return server, nil
	} else {
		return nil, errors.Errorf("proxy server %s doesn't exist", tag)
	}
}

// Servers is used to get all proxy servers
func (m *Manager) Servers() map[string]*Server {
	servers := make(map[string]*Server)
	m.rwm.RLock()
	for tag, server := range m.servers {
		servers[tag] = server
	}
	m.rwm.RUnlock()
	return servers
}

func (m *Manager) Close() error {
	var err error
	for _, server := range m.Servers() {
		e := server.Close()
		if err == nil {
			err = e
		}
	}
	return err
}

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

type Manager struct {
	logger  logger.Logger
	servers map[string]*Server // key = tag
	rwm     sync.RWMutex
}

// NewManager is used to create a proxy server manager
func NewManager() *Manager {
	return &Manager{servers: make(map[string]*Server)}
}

// Add is used to add proxy server, but not listen or serve
func (m *Manager) Add(tag string, server *Server) error {
	if tag == "" {
		return errors.New("empty proxy server tag")
	}
	deleteServer := func() {
		m.rwm.Lock()
		delete(m.servers, tag)
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
		s, err := socks.NewServer(tag, m.logger, opts)
		if err != nil {
			return err
		}
		server.server = s
	case ModeHTTP:
		opts := new(http.Options)
		err := toml.Unmarshal([]byte(server.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
		opts.ExitFunc = deleteServer
		s, err := http.NewServer(tag, m.logger, opts)
		if err != nil {
			return err
		}
		server.server = s
	default:
		return errors.Errorf("unknown mode %s", server.Mode)
	}
	server.CreateAt = time.Now()
	m.rwm.Lock()
	defer m.rwm.Unlock()
	if _, ok := m.servers[tag]; !ok {
		m.servers[tag] = server
		return nil
	} else {
		return errors.Errorf("proxy server %s already exists", tag)
	}
}

// Close is used to close proxy server
func (m *Manager) Close(tag string) error {
	if tag == "" {
		return errors.New("empty proxy server tag")
	}
	m.rwm.Lock()
	defer m.rwm.Unlock()
	if server, ok := m.servers[tag]; ok {
		// if server not serve
		m.rwm.Lock()
		delete(m.servers, tag)
		m.rwm.Unlock()
		return server.Close()
	} else {
		return errors.Errorf("proxy server %s doesn't exist", tag)
	}
}

// Get is used to get proxy server
func (m *Manager) Get(tag string) (*Server, error) {
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

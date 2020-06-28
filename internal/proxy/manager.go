package proxy

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/cert"
	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/patch/toml"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
)

// Manager is a proxy server manager.
type Manager struct {
	certPool *cert.Pool
	logger   logger.Logger
	now      func() time.Time

	// key = server tag
	servers map[string]*Server
	closed  bool
	rwm     sync.RWMutex
}

// NewManager is used to create a proxy server manager.
func NewManager(pool *cert.Pool, logger logger.Logger, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{
		certPool: pool,
		logger:   logger,
		now:      now,
		servers:  make(map[string]*Server),
	}
}

// Add is used to add proxy server, but not listen or serve.
func (m *Manager) Add(server *Server) error {
	err := m.add(server)
	if err != nil {
		const format = "failed to add proxy server %s"
		return errors.WithMessagef(err, format, server.Tag)
	}
	return nil
}

func (m *Manager) add(server *Server) error {
	if server.Tag == "" {
		return errors.New("empty proxy server tag")
	}
	var err error
	switch server.Mode {
	case ModeSocks5, ModeSocks4a, ModeSocks4:
		err = m.addSocks(server)
	case ModeHTTP, ModeHTTPS:
		err = m.addHTTP(server)
	default:
		return errors.Errorf("unknown mode: %s", server.Mode)
	}
	if err != nil {
		return err
	}
	server.now = m.now
	server.createAt = m.now()
	m.rwm.Lock()
	defer m.rwm.Unlock()
	if m.closed {
		return errors.New("proxy server manager closed")
	}
	if _, ok := m.servers[server.Tag]; !ok {
		m.servers[server.Tag] = server
		return nil
	}
	return errors.New("already exists")
}

func (m *Manager) addSocks(server *Server) error {
	opts := new(socks.Options)
	if server.Options != "" {
		err := toml.Unmarshal([]byte(server.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	opts.DialContext = server.DialContext
	// because the tag is never empty, socks.NewServer will not return error
	switch server.Mode {
	case ModeSocks5:
		server.server, _ = socks.NewSocks5Server(server.Tag, m.logger, opts)
	case ModeSocks4a:
		server.server, _ = socks.NewSocks4aServer(server.Tag, m.logger, opts)
	case ModeSocks4:
		server.server, _ = socks.NewSocks4Server(server.Tag, m.logger, opts)
	}
	return nil
}

func (m *Manager) addHTTP(server *Server) error {
	opts := new(http.Options)
	if server.Options != "" {
		err := toml.Unmarshal([]byte(server.Options), opts)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	opts.DialContext = server.DialContext
	var err error
	switch server.Mode {
	case ModeHTTP:
		server.server, err = http.NewHTTPServer(server.Tag, m.logger, opts)
	case ModeHTTPS:
		opts.Server.TLSConfig.CertPool = m.certPool
		opts.Transport.TLSClientConfig.CertPool = m.certPool
		server.server, err = http.NewHTTPSServer(server.Tag, m.logger, opts)
	}
	return err
}

// Delete is used to delete proxy server.
func (m *Manager) Delete(tag string) error {
	if tag == "" {
		return errors.New("empty proxy server tag")
	}
	m.rwm.Lock()
	defer m.rwm.Unlock()
	if server, ok := m.servers[tag]; ok {
		delete(m.servers, tag)
		return server.Close()
	}
	return errors.Errorf("proxy server %s doesn't exist", tag)
}

// Get is used to get proxy server.
func (m *Manager) Get(tag string) (*Server, error) {
	if tag == "" {
		return nil, errors.New("empty proxy server tag")
	}
	m.rwm.RLock()
	defer m.rwm.RUnlock()
	if server, ok := m.servers[tag]; ok {
		return server, nil
	}
	return nil, errors.Errorf("proxy server %s doesn't exist", tag)
}

// Servers is used to get all proxy servers.
func (m *Manager) Servers() map[string]*Server {
	m.rwm.RLock()
	defer m.rwm.RUnlock()
	servers := make(map[string]*Server, len(m.servers))
	for tag, server := range m.servers {
		servers[tag] = server
	}
	return servers
}

// Close is used to close all proxy servers.
func (m *Manager) Close() error {
	m.rwm.Lock()
	defer m.rwm.Unlock()
	if m.closed {
		return nil
	}
	var err error
	for tag, server := range m.servers {
		e := server.Close()
		if e != nil && !nettool.IsNetClosingError(e) && err == nil {
			err = e
		}
		delete(m.servers, tag)
	}
	m.closed = true
	return err
}

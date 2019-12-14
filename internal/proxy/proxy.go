package proxy

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

// support mode
const (
	ModeDirect  = "direct"
	ModeSocks   = "socks"
	ModeHTTP    = "http"
	ModeChain   = "chain"
	ModeBalance = "balance"
)

type client interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Connect(ctx context.Context, conn net.Conn, network, address string) (net.Conn, error)
	HTTP(t *http.Transport)
	Timeout() time.Duration
	Server() (network string, address string)
	Info() string
}

type server interface {
	ListenAndServe(network, address string) error
	Serve(l net.Listener)
	Close() error
	Address() string
	Info() string
}

// Client is proxy client
type Client struct {
	Tag     string `toml:"tag"`
	Mode    string `toml:"mode"`
	Network string `toml:"network"`
	Address string `toml:"address"`
	Options string `toml:"options"`
	client
}

// Server is proxy server
type Server struct {
	Tag     string `toml:"tag"`
	Mode    string `toml:"mode"`
	Options string `toml:"options"`
	server

	now       func() time.Time
	createAt  time.Time
	serveAt   time.Time
	rwm       sync.RWMutex
	serveOnce sync.Once
	closeOnce sync.Once
}

// ListenAndServe is used to listen a listener and serve
// it will not block
func (s *Server) ListenAndServe(network, address string) (err error) {
	s.serveOnce.Do(func() {
		err = s.server.ListenAndServe(network, address)
		s.rwm.Lock()
		s.serveAt = s.now()
		s.rwm.Unlock()
	})
	return
}

// Serve accepts incoming connections on the listener
// it will not block
func (s *Server) Serve(l net.Listener) {
	s.serveOnce.Do(func() {
		s.server.Serve(l)
		s.rwm.Lock()
		s.serveAt = s.now()
		s.rwm.Unlock()
	})
}

// CreateAt is used get proxy server create time
func (s *Server) CreateAt() time.Time {
	return s.createAt
}

// ServeAt is used get proxy server serve time
func (s *Server) ServeAt() time.Time {
	s.rwm.RLock()
	t := s.serveAt
	s.rwm.RUnlock()
	return t
}

// Close is used to close proxy server
func (s *Server) Close() (err error) {
	s.closeOnce.Do(func() { err = s.server.Close() })
	return
}

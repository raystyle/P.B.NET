package proxy

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

// supported modes
const (
	// basic
	ModeSocks5  = "socks5"
	ModeSocks4a = "socks4a"
	ModeSocks4  = "socks4"
	ModeHTTP    = "http"
	ModeHTTPS   = "https"

	// combine proxy client, include basic proxy client
	ModeChain   = "chain"
	ModeBalance = "balance"

	// reserve proxy client in Pool
	ModeDirect = "direct"
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
	Serve(l net.Listener) error
	Addresses() []net.Addr
	Info() string
	Close() error
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

	// secondary proxy
	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `toml:"-"`

	server

	now      func() time.Time
	createAt time.Time
	serveAt  []time.Time
	rwm      sync.RWMutex
}

func (s *Server) addServeAt() {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	s.serveAt = append(s.serveAt, s.now())
}

// ListenAndServe is used to listen a listener and serve
func (s *Server) ListenAndServe(network, address string) error {
	s.addServeAt()
	return s.server.ListenAndServe(network, address)
}

// Serve accepts incoming connections on the listener
func (s *Server) Serve(listener net.Listener) error {
	s.addServeAt()
	return s.server.Serve(listener)
}

// CreateAt is used get proxy server create time
func (s *Server) CreateAt() time.Time {
	return s.createAt
}

// ServeAt is used get proxy server serve time
func (s *Server) ServeAt() []time.Time {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return s.serveAt
}

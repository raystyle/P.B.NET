package proxy

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

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
	Connect(conn net.Conn, network, address string) (net.Conn, error)
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

type Client struct {
	Mode    string
	Network string
	Address string
	Options string
	tag     string
	client
}

type Server struct {
	Mode    string
	Options string
	server

	CreateAt  time.Time // maybe inaccurate
	serveAt   time.Time
	rwm       sync.RWMutex
	serveOnce sync.Once
	closeOnce sync.Once
}

func (s *Server) ListenAndServe(network, address string) (err error) {
	s.serveOnce.Do(func() {
		err = s.server.ListenAndServe(network, address)
		s.rwm.Lock()
		s.serveAt = time.Now()
		s.rwm.Unlock()
	})
	return
}

func (s *Server) Serve(l net.Listener) {
	s.serveOnce.Do(func() {
		s.server.Serve(l)
		s.rwm.Lock()
		s.serveAt = time.Now()
		s.rwm.Unlock()
	})
}

func (s *Server) Close() (err error) {
	s.closeOnce.Do(func() { err = s.server.Close() })
	return
}

func (s *Server) ServeAt() time.Time {
	s.rwm.RLock()
	t := s.serveAt
	s.rwm.RUnlock()
	return t
}

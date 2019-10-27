package proxy

import (
	"context"
	"net"
	"net/http"
	"time"
)

const (
	ModeDirect = "direct"
	ModeSocks  = "socks"
	ModeHTTP   = "http"
)

type Client struct {
	Mode    string
	Network string
	Address string
	Options string
	client
}

type Server struct {
	Mode    string
	Network string
	Address string
	Options string
	server
}

type client interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Connect(conn net.Conn, network, address string) error
	HTTP(t *http.Transport)
	Timeout() time.Duration
	Info() string
}

type server interface {
	ListenAndServe(network, address string) error
	Serve(l net.Listener)
	Close() error
	Address() string
	Info() string
}

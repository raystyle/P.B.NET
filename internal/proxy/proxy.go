package proxy

import (
	"context"
	"net"
	"net/http"
	"time"
)

type Mode string

const (
	SOCKS5 Mode = "socks5"
	HTTP   Mode = "http"
)

type Client interface {
	Dial(network, address string) (net.Conn, error)
	Dial_Context(ctx context.Context, network, address string) (net.Conn, error)
	Dial_Timeout(network, address string, timeout time.Duration) (net.Conn, error)
	HTTP(transport *http.Transport)
	Info() string
}

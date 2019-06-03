package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"project/internal/proxy/httpproxy"
	"project/internal/proxy/socks5"
)

type Mode uint8

const (
	SOCKS5 Mode = iota
	HTTP
)

var (
	ERR_UNKNOWN_MODE              = errors.New("unknown mode")
	ERR_INVALID_SOCKS5_CONFIG     = errors.New("invalid socks5 config")
	ERR_INVALID_HTTP_PROXY_CONFIG = errors.New("invalid http proxy config")
)

type Client interface {
	Dial(network, address string) (net.Conn, error)
	Dial_Context(ctx context.Context, network, address string) (net.Conn, error)
	Dial_Timeout(network, address string, timeout time.Duration) (net.Conn, error)
	HTTP(transport *http.Transport)
	Info() string
}

type Server interface {
	Start() error
	Stop() error
	Info() string
}

func Load_Client(mode Mode, config interface{}) (Client, error) {
	switch mode {
	case SOCKS5:
		if config, ok := config.([]*socks5.Config); ok {
			return socks5.New_Client(config...)
		}
		return nil, ERR_INVALID_SOCKS5_CONFIG
	case HTTP:
		if config, ok := config.(string); ok {
			return httpproxy.New_Client(config)
		}
		return nil, ERR_INVALID_HTTP_PROXY_CONFIG
	default:
		return nil, ERR_UNKNOWN_MODE
	}
}

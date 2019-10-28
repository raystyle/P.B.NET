package direct

import (
	"context"
	"net"
	"net/http"
	"time"

	"project/internal/options"
)

// Direct implement internal/proxy.client
type Direct struct{}

func (d Direct) Dial(network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: options.DefaultDialTimeout}).Dial(network, address)
}

func (d Direct) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: options.DefaultDialTimeout}).DialContext(ctx, network, address)
}

func (d Direct) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	return (&net.Dialer{Timeout: timeout}).Dial(network, address)
}

func (d Direct) Connect(conn net.Conn, _, _ string) (net.Conn, error) {
	return conn, nil
}

func (d Direct) HTTP(_ *http.Transport) {}

func (d Direct) Timeout() time.Duration {
	return options.DefaultDialTimeout
}

func (d Direct) Server() (string, string) {
	return "", ""
}

func (d Direct) Info() string {
	return "direct"
}

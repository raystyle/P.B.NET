package direct

import (
	"context"
	"net"
	"net/http"
	"time"
)

const dialTimeout = 30 * time.Second

// Direct implement internal/proxy.client.
type Direct struct{}

// Dial is used to Dial with default dial timeout.
func (d Direct) Dial(network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: dialTimeout}).Dial(network, address)
}

// DialContext is used to DialContext with default dial timeout.
func (d Direct) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, network, address)
}

// DialTimeout is used to Dial with timeout, if timeout < 1, use the default dial timeout.
func (d Direct) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = dialTimeout
	}
	return (&net.Dialer{Timeout: timeout}).Dial(network, address)
}

// Connect is a padding function.
func (d Direct) Connect(_ context.Context, conn net.Conn, _, _ string) (net.Conn, error) {
	return conn, nil
}

// HTTP is a padding function.
func (d Direct) HTTP(_ *http.Transport) {}

// Timeout is used to return timeout.
func (d Direct) Timeout() time.Duration {
	return dialTimeout
}

// Server is a padding function.
func (d Direct) Server() (string, string) {
	return "", ""
}

// Info is a padding function.
func (d Direct) Info() string {
	return "direct"
}

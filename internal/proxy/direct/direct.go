package direct

import (
	"context"
	"net"
	"net/http"
	"time"

	"project/internal/options"
)

type Direct struct{}

func (d Direct) Dial(network, address string) (net.Conn, error) {
	return new(net.Dialer).Dial(network, address)
}

func (d Direct) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return new(net.Dialer).DialContext(ctx, network, address)
}

func (d Direct) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	return (&net.Dialer{Timeout: timeout}).Dial(network, address)
}

func (d Direct) HTTP(_ *http.Transport) {}

func (d Direct) Info() string {
	return "Direct"
}

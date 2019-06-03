package direct

import (
	"context"
	"net"
	"net/http"
	"time"

	"project/internal/options"
)

type Direct struct{}

func (this *Direct) Dial(network, address string) (net.Conn, error) {
	return new(net.Dialer).Dial(network, address)
}

func (this *Direct) Dial_Context(ctx context.Context, network, address string) (net.Conn, error) {
	return new(net.Dialer).DialContext(ctx, network, address)
}

func (this *Direct) Dial_Timeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DEFAULT_DIAL_TIMEOUT
	}
	return (&net.Dialer{Timeout: timeout}).Dial(network, address)
}

func (this *Direct) HTTP(*http.Transport) {}

func (this *Direct) Info() string {
	return "Direct"
}

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
)

// Chain implement Client
type Chain struct {
	name    string
	clients []*Client // must > 0
	length  int
}

func NewChain(name string, clients ...*Client) (*Chain, error) {
	if name == "" {
		return nil, errors.New("empty proxy chain name")
	}
	l := len(clients)
	if l == 0 {
		return nil, errors.New("proxy chain need at least one proxy client")
	}
	c := make([]*Client, l)
	copy(c, clients)
	return &Chain{name: name, clients: c, length: l}, nil
}

func (c *Chain) Dial(network, address string) (net.Conn, error) {
	first := c.clients[0]
	fNetwork, fAddress := first.Network, first.Address
	conn, err := (&net.Dialer{Timeout: first.Timeout()}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.name, fAddress)
	}
	err = c.Connect(conn, network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "proxy chain %s", c.name)
	}
	return conn, nil
}

func (c *Chain) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	first := c.clients[0]
	fNetwork, fAddress := first.Network, first.Address
	conn, err := (&net.Dialer{Timeout: first.Timeout()}).DialContext(ctx, fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial context: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.name, fAddress)
	}
	err = c.Connect(conn, network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "proxy chain %s", c.name)
	}
	return conn, nil
}

func (c *Chain) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	fNetwork, fAddress := c.clients[0].Network, c.clients[0].Address
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial timeout: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.name, fAddress)
	}
	err = c.Connect(conn, network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "proxy chain %s", c.name)
	}
	return conn, nil
}

func (c *Chain) Connect(conn net.Conn, network, address string) error {
	var (
		err  error
		next int
	)
	for i := 0; i < c.length; i++ {
		next = i + 1
		if next != c.length {
			nextNetwork := c.clients[next].Network
			nextAddress := c.clients[next].Address
			err = c.clients[i].Connect(conn, nextNetwork, nextAddress)
			if err != nil {
				return err
			}
		} else {
			break
		}
	}
	return c.clients[next-1].Connect(conn, network, address)
}

func (c *Chain) HTTP(t *http.Transport) {
	t.DialContext = c.DialContext
}

func (c *Chain) Timeout() time.Duration {
	var timeout time.Duration
	for i := 0; i < c.length; i++ {
		timeout += c.clients[i].Timeout()
	}
	return timeout
}

// Info is used to get the proxy chain info
// proxy chain: name
// http://admin:123456@127.0.0.1:8080
// socks5 tcp 127.0.0.1:1080 admin 123456
// socks4a tcp 127.0.0.1:1081
func (c *Chain) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("proxy chain: ")
	buf.WriteString(c.name)
	for i := 0; i < c.length; i++ {
		_, _ = fmt.Fprintf(buf, "\n%s", c.clients[i].Info())
	}
	return buf.String()
}

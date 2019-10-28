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
	tag     string
	clients []*Client // must > 0
	length  int

	first *Client
}

func NewChain(tag string, clients ...*Client) (*Chain, error) {
	if tag == "" {
		return nil, errors.New("empty proxy chain tag")
	}
	l := len(clients)
	if l == 0 {
		return nil, errors.New("proxy chain need at least one proxy client")
	}
	cs := make([]*Client, l)
	copy(cs, clients)
	return &Chain{
		tag:     tag,
		clients: cs,
		length:  l,
		first:   cs[0],
	}, nil
}

func (c *Chain) Dial(network, address string) (net.Conn, error) {
	fTimeout := c.first.Timeout()
	fNetwork, fAddress := c.first.Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := c.Connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return pConn, nil
}

func (c *Chain) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	fTimeout := c.first.Timeout()
	fNetwork, fAddress := c.first.Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).DialContext(ctx, fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial context: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := c.Connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return pConn, nil
}

func (c *Chain) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	fNetwork, fAddress := c.first.Server()
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial timeout: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := c.Connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return pConn, nil
}

func (c *Chain) Connect(conn net.Conn, network, address string) (net.Conn, error) {
	conn, err := c.connect(conn, network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "proxy chain %s", c.tag)
	}
	return conn, nil
}

func (c *Chain) connect(conn net.Conn, network, address string) (net.Conn, error) {
	var (
		err  error
		next int
	)
	for i := 0; i < c.length; i++ {
		next = i + 1
		if next < c.length {
			nextNetwork, nextAddress := c.clients[next].Server()
			conn, err = c.clients[i].Connect(conn, nextNetwork, nextAddress)
			if err != nil {
				return nil, err
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

func (c *Chain) Server() (string, string) {
	return c.first.Server()
}

// Info is used to get the proxy chain info
// proxy chain: tag
// 1. tag-a: http://admin:123456@127.0.0.1:8080
// 2. tag-b: socks5 tcp 127.0.0.1:1080 admin 123456
// 3. tag-c: socks4a tcp 127.0.0.1:1081
func (c *Chain) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("proxy chain: ")
	buf.WriteString(c.tag)
	for i := 0; i < c.length; i++ {
		c := c.clients[i]
		_, _ = fmt.Fprintf(buf, "\n%d. %s: %s", i+1, c.tag, c.Info())
	}
	return buf.String()
}

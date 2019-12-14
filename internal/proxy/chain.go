package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

const defaultDialTimeout = 30 * time.Second

// Chain implement client
type Chain struct {
	tag     string
	clients []*Client // must > 0
	length  int

	first *Client
}

// NewChain is used to create a proxy chain
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

// Dial is used to connect to address through proxy chain
func (c *Chain) Dial(network, address string) (net.Conn, error) {
	fTimeout := c.first.Timeout()
	fNetwork, fAddress := c.first.Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := c.Connect(context.Background(), conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialContext is used to connect to address through proxy chain with context
func (c *Chain) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	fTimeout := c.first.Timeout()
	fNetwork, fAddress := c.first.Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).DialContext(ctx, fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial context: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := c.Connect(ctx, conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialTimeout is used to connect to address through proxy chain with timeout
func (c *Chain) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	fNetwork, fAddress := c.first.Server()
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "proxy chain %s dial timeout: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := c.Connect(context.Background(), conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// Connect is used to connect to address through proxy chain with context
func (c *Chain) Connect(
	ctx context.Context,
	conn net.Conn,
	network string,
	address string,
) (net.Conn, error) {
	conn, err := c.connect(ctx, conn, network, address)
	return conn, errors.WithMessagef(err, "proxy chain %s", c.tag)
}

func (c *Chain) connect(
	ctx context.Context,
	conn net.Conn,
	network string,
	address string,
) (net.Conn, error) {
	var (
		next int
		err  error
	)
	for i := 0; i < c.length; i++ {
		next = i + 1
		if next < c.length {
			nextNetwork, nextAddress := c.clients[next].Server()
			conn, err = c.clients[i].Connect(ctx, conn, nextNetwork, nextAddress)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	return c.clients[next-1].Connect(ctx, conn, network, address)
}

// HTTP is used to set *http.Transport about proxy
func (c *Chain) HTTP(t *http.Transport) {
	t.DialContext = c.DialContext
}

// Timeout is used to get the proxy chain timeout
func (c *Chain) Timeout() time.Duration {
	var timeout time.Duration
	for i := 0; i < c.length; i++ {
		timeout += c.clients[i].Timeout()
	}
	return timeout
}

// Server is used to get the first proxy client related proxy server address
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
		_, _ = fmt.Fprintf(buf, "\n%d. %s: %s", i+1, c.Tag, c.Info())
	}
	return buf.String()
}

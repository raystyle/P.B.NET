package socks

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/xnet/xnetutil"
)

const defaultDialTimeout = 30 * time.Second

// Client implement internal/proxy.client
type Client struct {
	network    string
	address    string
	username   string
	password   string
	timeout    time.Duration
	socks4     bool
	userID     []byte
	disableExt bool // socks4A remote hostname resolving feature

	protocol string
	info     string
}

// NewClient is used to create socks client
func NewClient(network, address string, opts *Options) (*Client, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}

	if opts == nil {
		opts = new(Options)
	}

	c := Client{
		network:    network,
		address:    address,
		username:   opts.Username,
		password:   opts.Password,
		timeout:    opts.Timeout,
		socks4:     opts.Socks4,
		userID:     []byte(opts.UserID),
		disableExt: opts.DisableSocks4A,
	}

	if c.timeout < 1 {
		c.timeout = defaultDialTimeout
	}

	switch {
	case !c.socks4:
		c.protocol = "socks5"
	case c.socks4 && opts.DisableSocks4A:
		c.protocol = "socks4"
	default:
		c.protocol = "socks4a"
	}

	if c.username != "" {
		c.info = fmt.Sprintf("%s %s %s %s:%s",
			c.protocol, c.network, c.address, c.username, c.password)
	} else {
		c.info = fmt.Sprintf("%s %s %s", c.protocol, c.network, c.address) // TODO ID
	}
	return &c, nil
}

// Dial is used to connect to address through proxy
func (c *Client) Dial(network, address string) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	conn, err := (&net.Dialer{Timeout: c.timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial: failed to connect %s server %s"
		return nil, errors.Wrapf(err, format, c.protocol, c.address)
	}
	_, err = c.Connect(context.Background(), conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial: %s server %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.protocol, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// DialContext is used to connect to address through proxy with context
func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	conn, err := (&net.Dialer{Timeout: c.timeout}).DialContext(ctx, c.network, c.address)
	if err != nil {
		const format = "dial context: failed to connect %s server %s"
		return nil, errors.Wrapf(err, format, c.protocol, c.address)
	}
	_, err = c.Connect(ctx, conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial context: %s server %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.protocol, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// DialTimeout is used to connect to address through proxy with timeout
func (c *Client) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial timeout: failed to connect %s server %s"
		return nil, errors.Wrapf(err, format, c.protocol, c.address)
	}
	_, err = c.Connect(context.Background(), conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial timeout: %s server %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.protocol, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// Connect is used to connect to address through proxy with context
func (c *Client) Connect(
	ctx context.Context,
	conn net.Conn,
	network string,
	address string,
) (net.Conn, error) {
	// check network
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupported network: %s", network)
	}
	host, port, err := splitHostPort(address)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// interrupt
	wg := sync.WaitGroup{}
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer func() {
			recover()
			wg.Done()
		}()
		select {
		case <-done:
		case <-ctx.Done():
			_ = conn.Close()
		}
	}()
	defer func() {
		close(done)
		wg.Wait()
	}()

	// connect
	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	if c.socks4 {
		err = c.connectSocks4(conn, host, port)
	} else {
		err = c.connectSocks5(conn, host, port)
	}
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// HTTP is used to set *http.Transport about proxy
func (c *Client) HTTP(t *http.Transport) {
	t.DialContext = c.DialContext
}

// Timeout is used to get the socks client timeout
func (c *Client) Timeout() time.Duration {
	return c.timeout
}

// Server is used to get the socks server address
func (c *Client) Server() (string, string) {
	return c.network, c.address
}

// Info is used to get the socks client info
//
// socks5 tcp 127.0.0.1:1080 admin:123456
// socks4a tcp 127.0.0.1:1080
func (c *Client) Info() string {
	return c.info
}

func splitHostPort(address string) (string, uint16, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	err = xnetutil.CheckPort(portNum)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	return host, uint16(portNum), nil
}

package socks

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"project/internal/nettool"
	"project/internal/xpanic"
)

// Client implemented internal/proxy.client.
type Client struct {
	network    string
	address    string
	socks4     bool
	disableExt bool // socks4, disable resolve domain name

	// options
	username []byte
	password []byte
	userID   []byte
	timeout  time.Duration

	protocol string // "socks5", "socks4a", "socks4"
	info     string
}

// NewSocks5Client is used to create a socks5 client.
func NewSocks5Client(network, address string, opts *Options) (*Client, error) {
	return newClient(network, address, opts, false, false)
}

// NewSocks4aClient is used to create a socks4a client.
func NewSocks4aClient(network, address string, opts *Options) (*Client, error) {
	return newClient(network, address, opts, true, false)
}

// NewSocks4Client is used to create a socks4 client.
func NewSocks4Client(network, address string, opts *Options) (*Client, error) {
	return newClient(network, address, opts, true, true)
}

func newClient(network, address string, opts *Options, socks4, disableExt bool) (*Client, error) {
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = new(Options)
	}
	client := Client{
		network:    network,
		address:    address,
		socks4:     socks4,
		disableExt: disableExt,
		username:   []byte(opts.Username),
		password:   []byte(opts.Password),
		userID:     []byte(opts.UserID),
		timeout:    opts.Timeout,
	}
	if client.timeout < 1 {
		client.timeout = defaultDialTimeout
	}
	// switch protocol
	switch {
	case !client.socks4:
		client.protocol = "socks5"
	case client.socks4 && disableExt:
		client.protocol = "socks4"
	default:
		client.protocol = "socks4a"
	}
	// info
	buf := new(bytes.Buffer)
	const format = "%-7s %s %s"
	_, _ = fmt.Fprintf(buf, format, client.protocol, client.network, client.address)
	if client.protocol == "socks5" {
		if opts.Username != "" {
			const format = " auth: %s:%s"
			_, _ = fmt.Fprintf(buf, format, client.username, client.password)
		}
	} else {
		if opts.UserID != "" {
			const format = " user id: %s"
			_, _ = fmt.Fprintf(buf, format, client.userID)
		}
	}
	client.info = buf.String()
	return &client, nil
}

// Dial is used to connect to address through proxy.
func (c *Client) Dial(network, address string) (net.Conn, error) {
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
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

// DialContext is used to connect to address through proxy with context.
func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
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

// DialTimeout is used to connect to address through proxy with timeout.
func (c *Client) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
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

// Connect is used to connect to address through proxy with context.
func (c *Client) Connect(ctx context.Context, conn net.Conn, network, address string) (net.Conn, error) {
	err := CheckNetwork(network)
	if err != nil {
		return nil, err
	}
	host, port, err := nettool.SplitHostPort(address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// interrupt
	var errCh chan error
	if ctx.Done() != nil {
		errCh = make(chan error, 1)
	}
	if errCh == nil {
		if c.socks4 {
			err = c.connectSocks4(conn, host, port)
		} else {
			err = c.connectSocks5(conn, host, port)
		}
	} else {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					b := xpanic.Log(r, "Client.Connect")
					errCh <- errors.New(b.String())
				}
				close(errCh)
			}()
			if c.socks4 {
				errCh <- c.connectSocks4(conn, host, port)
			} else {
				errCh <- c.connectSocks5(conn, host, port)
			}
		}()
		select {
		case err = <-errCh:
			if err != nil {
				// if the error was due to the context
				// closing, prefer the context's error, rather
				// than some random network teardown error.
				if e := ctx.Err(); e != nil {
					err = e
				}
			}
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// HTTP is used to set *http.Transport about proxy.
func (c *Client) HTTP(t *http.Transport) {
	t.DialContext = c.DialContext
}

// Timeout is used to get the socks client timeout.
func (c *Client) Timeout() time.Duration {
	return c.timeout
}

// Server is used to get the socks server address.
func (c *Client) Server() (string, string) {
	return c.network, c.address
}

// Info is used to get the socks client information.
//
// socks5  tcp 127.0.0.1:1080 auth: admin:123456
// socks4a tcp 127.0.0.1:1080 user id: test
func (c *Client) Info() string {
	return c.info
}

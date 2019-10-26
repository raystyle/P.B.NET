package socks

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"project/internal/options"
	"project/internal/xnet/xnetutil"
)

// Client implement internal/proxy.Client
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

func NewClient(network, address string, socks4 bool, opts *Options) (*Client, error) {
	// check network
	switch network {
	case "", "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.Errorf("unsupport network: %s", network)
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
		socks4:     socks4,
		userID:     []byte(opts.UserID),
		disableExt: opts.DisableSocks4A,
	}

	if c.timeout < 1 {
		c.timeout = options.DefaultDialTimeout
	}

	switch {
	case !socks4:
		c.protocol = "socks5"
	case socks4 && opts.DisableSocks4A:
		c.protocol = "socks4"
	default:
		c.protocol = "socks4a"
	}

	if c.username != "" {
		c.info = fmt.Sprintf("%s %s %s %s:%s",
			c.protocol, c.network, c.address, c.username, c.password)
	} else {
		c.info = fmt.Sprintf("%s %s %s", c.protocol, c.network, c.address)
	}
	return &c, nil
}

func (c *Client) Dial(network, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial: failed to connect %s server %s"
		return nil, errors.Wrapf(err, format, c.protocol, c.address)
	}
	err = c.Connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial: %s server %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.protocol, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).DialContext(ctx, c.network, c.address)
	if err != nil {
		const format = "dial context: failed to connect %s server %s"
		return nil, errors.Wrapf(err, format, c.protocol, c.address)
	}
	err = c.Connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial context: %s server %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.protocol, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial timeout: failed to connect %s server %s"
		return nil, errors.Wrapf(err, format, c.protocol, c.address)
	}
	err = c.Connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		const format = "dial timeout: %s server %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.protocol, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) Connect(conn net.Conn, _, address string) error {
	if c.socks4 {
		return c.connectSocks4(conn, "", address)
	}
	return c.connectSocks5(conn, "", address)
}

func (c *Client) HTTP(t *http.Transport) {
	t.DialContext = c.DialContext
}

func (c *Client) Timeout() time.Duration {
	return c.timeout
}

func (c *Client) Address() (string, string) {
	return c.network, c.address
}

// Info is used to get the proxy info
// socks5 tcp 127.0.0.1:1080 admin 123456
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

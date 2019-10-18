package socks5

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/proxy/direct"
	"project/internal/xnet"
)

type Config struct {
	Network  string `toml:"network"`
	Address  string `toml:"address"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// support proxy chain
type Client struct {
	dial        func(network, address string) (net.Conn, error)
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	dialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)
	chain       string
}

func NewClient(c ...*Config) (*Client, error) {
	d := direct.Direct{}
	s := &Client{
		dial:        d.Dial,
		dialContext: d.DialContext,
		dialTimeout: d.DialTimeout,
		chain:       "direct",
	}
	for i := 0; i < len(c); i++ {
		err := s.add(c[i])
		if err != nil {
			return nil, errors.WithMessage(err, "add")
		}
	}
	return s, nil
}

func (c *Client) Dial(network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ErrNotSupportNetwork
	}
	return c.dial(network, address)
}

func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ErrNotSupportNetwork
	}
	return c.dialContext(ctx, network, address)
}

func (c *Client) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ErrNotSupportNetwork
	}
	return c.dialTimeout(network, address, timeout)
}

func (c *Client) HTTP(t *http.Transport) {
	t.DialContext = c.dialContext
}

func (c *Client) Info() string {
	return c.chain
}

func (c *Client) add(server *Config) error {
	switch server.Network {
	case "tcp", "tcp4", "tcp6":
	default:
		return ErrNotSupportNetwork
	}
	d := &dialer{
		server:      server,
		dial:        c.dial,
		dialContext: c.dialContext,
		dialTimeout: c.dialTimeout,
	}
	c.dial = d.Dial
	c.dialContext = d.DialContext
	c.dialTimeout = d.DialTimeout
	// update chain
	buffer := bytes.Buffer{}
	buffer.WriteString(c.chain)
	buffer.WriteString(" -> [")
	buffer.WriteString(server.Network)
	buffer.WriteString(" ")
	buffer.WriteString(server.Address)
	buffer.WriteString("]")
	c.chain = buffer.String()
	return nil
}

type dialer struct {
	server      *Config
	dial        func(network, address string) (net.Conn, error)
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	dialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)
}

func (d *dialer) Dial(network, address string) (net.Conn, error) {
	conn, err := d.dial(d.server.Network, d.server.Address)
	if err != nil {
		return nil, errors.WithMessagef(err, "Dial Socks5 Server %s failed",
			d.server.Address)
	}
	err = d.connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "Socks5 Server %s Connect %s failed",
			d.server.Address, address)
	}
	return conn, nil
}

func (d *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.dialContext(ctx, d.server.Network, d.server.Address)
	if err != nil {
		return nil, errors.WithMessagef(err, "Dial Socks5 Server %s failed",
			d.server.Address)
	}
	err = d.connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "Socks5 Server %s Connect %s failed",
			d.server.Address, address)
	}
	return conn, nil
}

func (d *dialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := d.dialTimeout(d.server.Network, d.server.Address, timeout)
	if err != nil {
		return nil, errors.WithMessagef(err, "Dial Socks5 Server %s failed",
			d.server.Address)
	}
	err = d.connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "Socks5 Server %s Connect %s failed",
			d.server.Address, address)
	}
	return conn, nil
}

// https://www.ietf.org/rfc/rfc1928.txt
func (d *dialer) connect(conn net.Conn, network, address string) error {
	host, port, err := splitHostPort(address)
	if err != nil {
		return err
	}
	conn = xnet.NewDeadlineConn(conn, 0)
	// request authentication
	buffer := bytes.Buffer{}
	buffer.WriteByte(version5)
	if d.server.Username == "" {
		buffer.WriteByte(1)
		buffer.WriteByte(notRequired)
	} else {
		buffer.WriteByte(2)
		buffer.WriteByte(notRequired)
		buffer.WriteByte(usernamePassword)
	}
	_, err = conn.Write(buffer.Bytes())
	if err != nil {
		return err
	}
	response := make([]byte, 2)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return err
	}
	if response[0] != version5 {
		return fmt.Errorf("unexpected protocol version %d", response[0])
	}
	am := response[1]
	if am == noAcceptableMethods {
		return ErrNoAcceptableMethods
	}
	// authenticate
	switch am {
	case notRequired:
	case usernamePassword:
		username := d.server.Username
		password := d.server.Password
		if len(username) == 0 || len(username) > 255 {
			return errors.New("invalid username length")
		}
		// https://www.ietf.org/rfc/rfc1929.txt
		buffer.Reset()
		buffer.WriteByte(usernamePasswordVersion)
		buffer.WriteByte(byte(len(username)))
		buffer.WriteString(username)
		buffer.WriteByte(byte(len(password)))
		buffer.WriteString(password)
		_, err := conn.Write(buffer.Bytes())
		if err != nil {
			return err
		}
		response := make([]byte, 2)
		_, err = io.ReadFull(conn, response)
		if err != nil {
			return err
		}
		if response[0] != usernamePasswordVersion {
			return errors.New("invalid username/password version")
		}
		if response[1] != statusSucceeded {
			return errors.New("invalid username/password")
		}
	default:
		return fmt.Errorf("unsupported authentication method %d", am)
	}
	// send connect target
	buffer.Reset()
	buffer.WriteByte(version5)
	buffer.WriteByte(connect)
	buffer.WriteByte(reserve)
	ip := net.ParseIP(host)
	if ip != nil {
		ip4 := ip.To4()
		if ip4 != nil {
			buffer.WriteByte(ipv4)
			buffer.Write(ip4)
		} else {
			ip6 := ip.To16()
			if ip6 != nil {
				buffer.WriteByte(ipv6)
				buffer.Write(ip6)
			} else {
				return errors.New("unknown address type")
			}
		}
	} else {
		if len(host) > 255 {
			return errors.New("FQDN too long")
		}
		buffer.WriteByte(fqdn)
		buffer.WriteByte(byte(len(host)))
		buffer.Write([]byte(host))
	}
	buffer.Write(convert.Uint16ToBytes(uint16(port)))
	_, err = conn.Write(buffer.Bytes())
	if err != nil {
		return err
	}
	// receive reply
	response = make([]byte, 4)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return err
	}
	if response[0] != version5 {
		return fmt.Errorf("unexpected protocol version %d", response[0])
	}
	if response[1] != succeeded {
		return errors.New(Reply(response[1]).String())
	}
	if response[2] != 0 {
		return errors.New("non-zero reserved field")
	}
	l := 2 // port
	switch response[3] {
	case ipv4:
		l += net.IPv4len
	case ipv6:
		l += net.IPv6len
	case fqdn:
		_, err = io.ReadFull(conn, response[:1])
		if err != nil {
			return err
		}
		l += int(response[0])
	}
	// grow
	if cap(response) < l {
		response = make([]byte, l)
	} else {
		response = response[:l]
	}
	_, err = io.ReadFull(conn, response)
	return err
}

func splitHostPort(address string) (string, int, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	err = xnet.CheckPort(portNum)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	return host, portNum, nil
}

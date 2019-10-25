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
	"project/internal/options"
	"project/internal/xnet/xnetutil"
)

// support proxy chain
type Client struct {
	network  string
	address  string
	username string
	password string
	timeout  time.Duration

	info string
}

func NewClient(network, address string, opts *Options) (*Client, error) {
	switch network {
	case "":
		network = "tcp"
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ErrNotSupportNetwork
	}
	if opts == nil {
		opts = new(Options)
	}
	client := Client{
		network:  network,
		address:  address,
		username: opts.Username,
		password: opts.Password,
		timeout:  opts.Timeout,
	}
	if client.timeout < 1 {
		client.timeout = options.DefaultDialTimeout
	}
	if client.username != "" {
		client.info = fmt.Sprintf("%s %s %s:%s",
			client.network, client.address, client.username, client.password)
	} else {
		client.info = fmt.Sprintf("%s %s", client.network, client.address)
	}
	return &client, nil
}

func (c *Client) Dial(_, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial: connect socks5 server %s failed"
		return nil, errors.Wrapf(err, format, c.address)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial: socks5 server %s connect %s failed"
		return nil, errors.WithMessagef(err, format, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) DialContext(ctx context.Context, _, address string) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.timeout}).DialContext(ctx, c.network, c.address)
	if err != nil {
		const format = "dial context: connect socks5 server %s failed"
		return nil, errors.Wrapf(err, format, c.address)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial context: socks5 server %s connect %s failed"
		return nil, errors.WithMessagef(err, format, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (c *Client) DialTimeout(_, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(c.network, c.address)
	if err != nil {
		const format = "dial timeout: connect socks5 server %s failed"
		return nil, errors.Wrapf(err, format, c.address)
	}
	err = c.Connect(conn, "", address)
	if err != nil {
		_ = conn.Close()
		const format = "dial timeout: socks5 server %s connect %s failed"
		return nil, errors.WithMessagef(err, format, c.address, address)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// https://www.ietf.org/rfc/rfc1928.txt
func (c *Client) Connect(conn net.Conn, _, address string) error {
	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	host, port, err := splitHostPort(address)
	if err != nil {
		return err
	}
	// request authentication
	buffer := bytes.Buffer{}
	buffer.WriteByte(version5)
	if c.username == "" {
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
		username := c.username
		password := c.password
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

func (c *Client) HTTP(t *http.Transport) {
	t.DialContext = c.DialContext
}

func (c *Client) Info() string {
	return c.info
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
	err = xnetutil.CheckPort(portNum)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	return host, portNum, nil
}

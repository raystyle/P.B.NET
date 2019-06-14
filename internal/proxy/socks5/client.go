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
	Network  string
	Address  string
	Username string
	Password string
}

// support proxy chain
type Client struct {
	dial         func(network, address string) (net.Conn, error)
	dial_context func(ctx context.Context, network, address string) (net.Conn, error)
	dial_timeout func(network, address string, timeout time.Duration) (net.Conn, error)
	chain        string
}

func New_Client(c ...*Config) (*Client, error) {
	d := direct.Direct{}
	s := &Client{
		dial:         d.Dial,
		dial_context: d.Dial_Context,
		dial_timeout: d.Dial_Timeout,
		chain:        "direct",
	}
	for i := 0; i < len(c); i++ {
		err := s.add(c[i])
		if err != nil {
			return nil, errors.WithMessage(err, "add")
		}
	}
	return s, nil
}

func (this *Client) Dial(network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ERR_NOT_SUPPORT_NETWORK
	}
	return this.dial(network, address)
}

func (this *Client) Dial_Context(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ERR_NOT_SUPPORT_NETWORK
	}
	return this.dial_context(ctx, network, address)
}

func (this *Client) Dial_Timeout(network, address string, timeout time.Duration) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ERR_NOT_SUPPORT_NETWORK
	}
	return this.dial_timeout(network, address, timeout)
}

func (this *Client) HTTP(t *http.Transport) {
	t.DialContext = this.dial_context
}

func (this *Client) Info() string {
	return this.chain
}

func (this *Client) add(server *Config) error {
	switch server.Network {
	case "tcp", "tcp4", "tcp6":
	default:
		return ERR_NOT_SUPPORT_NETWORK
	}
	d := &dialer{
		server:       server,
		dial:         this.dial,
		dial_context: this.dial_context,
		dial_timeout: this.dial_timeout,
	}
	this.dial = d.Dial
	this.dial_context = d.Dial_Context
	this.dial_timeout = d.Dial_Timeout
	// update chain
	buffer := bytes.Buffer{}
	buffer.WriteString(this.chain)
	buffer.WriteString(" -> [")
	buffer.WriteString(server.Network)
	buffer.WriteString(" ")
	buffer.WriteString(server.Address)
	buffer.WriteString("]")
	this.chain = buffer.String()
	return nil
}

type dialer struct {
	server       *Config
	dial         func(network, address string) (net.Conn, error)
	dial_context func(ctx context.Context, network, address string) (net.Conn, error)
	dial_timeout func(network, address string, timeout time.Duration) (net.Conn, error)
}

func (this *dialer) Dial(network, address string) (net.Conn, error) {
	conn, err := this.dial(this.server.Network, this.server.Address)
	if err != nil {
		return nil, errors.WithMessagef(err, "Dial Socks5 Server %s failed",
			this.server.Address)
	}
	err = this.connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "Socks5 Server %s Connect %s failed",
			this.server.Address, address)
	}
	return conn, nil
}

func (this *dialer) Dial_Context(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := this.dial_context(ctx, this.server.Network, this.server.Address)
	if err != nil {
		return nil, errors.WithMessagef(err, "Dial Socks5 Server %s failed",
			this.server.Address)
	}
	err = this.connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "Socks5 Server %s Connect %s failed",
			this.server.Address, address)
	}
	return conn, nil
}

func (this *dialer) Dial_Timeout(network, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := this.dial_timeout(this.server.Network, this.server.Address, timeout)
	if err != nil {
		return nil, errors.WithMessagef(err, "Dial Socks5 Server %s failed",
			this.server.Address)
	}
	err = this.connect(conn, network, address)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "Socks5 Server %s Connect %s failed",
			this.server.Address, address)
	}
	return conn, nil
}

// https://www.ietf.org/rfc/rfc1928.txt
func (this *dialer) connect(conn net.Conn, network, address string) error {
	host, port, err := split_host_port(address)
	if err != nil {
		return err
	}
	// request authentication
	buffer := bytes.Buffer{}
	buffer.WriteByte(version5)
	if this.server.Username == "" {
		buffer.WriteByte(1)
		buffer.WriteByte(not_required)
	} else {
		buffer.WriteByte(2)
		buffer.WriteByte(not_required)
		buffer.WriteByte(username_password)
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
	if am == no_acceptable_methods {
		return ERR_NO_ACCEPTABLE_METHODS
	}
	// authenticate
	switch am {
	case not_required:
	case username_password:
		username := this.server.Username
		password := this.server.Password
		if len(username) == 0 || len(username) > 255 {
			return errors.New("invalid username length")
		}
		//https://www.ietf.org/rfc/rfc1929.txt
		buffer.Reset()
		buffer.WriteByte(username_password_version)
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
		if response[0] != username_password_version {
			return errors.New("invalid username/password version")
		}
		if response[1] != status_succeeded {
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
	buffer.Write(convert.Uint16_Bytes(uint16(port)))
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

func split_host_port(address string) (string, int, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	portnum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	err = xnet.Inspect_Port_int(portnum)
	if err != nil {
		return "", 0, errors.WithStack(err)
	}
	return host, portnum, nil
}

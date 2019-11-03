package socks

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
)

const (
	version5 uint8 = 0x05
	// auth method
	notRequired         uint8 = 0x00
	usernamePassword    uint8 = 0x02
	noAcceptableMethods uint8 = 0xFF
	// auth
	usernamePasswordVersion uint8 = 0x01
	statusSucceeded         uint8 = 0x00
	statusFailed            uint8 = 0x01

	reserve   uint8 = 0x00
	noReserve uint8 = 0x01
	// cmd
	connect uint8 = 0x01
	// address
	ipv4 uint8 = 0x01
	fqdn uint8 = 0x03
	ipv6 uint8 = 0x04
	// reply
	succeeded      uint8 = 0x00
	connRefused    uint8 = 0x05
	cmdNotSupport  uint8 = 0x07
	addrNotSupport uint8 = 0x08
)

type v5Reply uint8

func (r v5Reply) String() string {
	switch r {
	case 0x01:
		return "general SOCKS server failure"
	case 0x02:
		return "connection not allowed by ruleset"
	case 0x03:
		return "network unreachable"
	case 0x04:
		return "host unreachable"
	case 0x05:
		return "connection refused"
	case 0x06:
		return "TTL expired"
	case 0x07:
		return "command not supported"
	case 0x08:
		return "address type not supported"
	default:
		return "unknown reply: " + strconv.Itoa(int(r))
	}
}

// https://www.ietf.org/rfc/rfc1928.txt
func (c *Client) connectSocks5(conn net.Conn, host string, port uint16) error {
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
	_, err := conn.Write(buffer.Bytes())
	if err != nil {
		return errors.WithStack(err)
	}
	response := make([]byte, 2)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return errors.WithStack(err)
	}
	if response[0] != version5 {
		return errors.Errorf("unexpected protocol version %d", response[0])
	}
	am := response[1]
	if am == noAcceptableMethods {
		return errors.New("no acceptable authentication methods")
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
			return errors.WithStack(err)
		}
		response := make([]byte, 2)
		_, err = io.ReadFull(conn, response)
		if err != nil {
			return errors.WithStack(err)
		}
		if response[0] != usernamePasswordVersion {
			return errors.New("invalid username/password version")
		}
		if response[1] != statusSucceeded {
			return errors.New("invalid username/password")
		}
	default:
		return errors.Errorf("unsupported authentication method %d", am)
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
			buffer.WriteByte(ipv6)
			buffer.Write(ip.To16())
		}
	} else {
		if len(host) > 255 {
			return errors.New("FQDN too long")
		}
		buffer.WriteByte(fqdn)
		buffer.WriteByte(byte(len(host)))
		buffer.Write([]byte(host))
	}
	buffer.Write(convert.Uint16ToBytes(port))
	_, err = conn.Write(buffer.Bytes())
	if err != nil {
		return errors.WithStack(err)
	}
	// receive reply
	response = make([]byte, 4)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return errors.WithStack(err)
	}
	if response[0] != version5 {
		return errors.Errorf("unexpected protocol version %d", response[0])
	}
	if response[1] != succeeded {
		return errors.New(v5Reply(response[1]).String())
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
			return errors.WithStack(err)
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
	return errors.WithStack(err)
}

var (
	v5ReplySucceeded         = []byte{version5, succeeded, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	v5ReplyConnectRefused    = []byte{version5, connRefused, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	v5ReplyAddressNotSupport = []byte{version5, addrNotSupport, reserve, ipv4, 0, 0, 0, 0, 0, 0}
)

func (c *conn) serveSocks5() {
	var err error
	defer func() {
		if err != nil {
			c.log(logger.Error, err)
		}
	}()
	buffer := make([]byte, 16) // prepare
	// read version
	_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
	if err != nil {
		return
	}
	if buffer[0] != version5 {
		c.log(logger.Error, "unexpected protocol version")
		return
	}
	// read authentication methods
	_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
	if err != nil {
		return
	}
	l := int(buffer[0])
	if l == 0 {
		c.log(logger.Error, "no authentication method")
		return
	}
	if l > len(buffer) {
		buffer = make([]byte, l)
	}
	_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
	if err != nil {
		return
	}
	// write authentication method
	if c.server.username != nil {
		_, err = c.conn.Write([]byte{version5, usernamePassword})
		if err != nil {
			return
		}
		// read username and password version
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		if buffer[0] != usernamePasswordVersion {
			c.log(logger.Error, "unexpected username password version")
			return
		}
		// read username length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read username
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			return
		}
		username := make([]byte, l)
		copy(username, buffer[:l])
		// read password length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		// read password
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			return
		}
		password := make([]byte, l)
		copy(password, buffer[:l])
		// write username password version
		_, err = c.conn.Write([]byte{usernamePasswordVersion})
		if err != nil {
			return
		}
		if subtle.ConstantTimeCompare(c.server.username, username) != 1 ||
			subtle.ConstantTimeCompare(c.server.password, password) != 1 {
			l := fmt.Sprintf("invalid username password: %s %s", username, password)
			c.log(logger.Exploit, l)
			_, _ = c.conn.Write([]byte{statusFailed})
			return
		} else {
			_, err = c.conn.Write([]byte{statusSucceeded})
		}
	} else {
		_, err = c.conn.Write([]byte{version5, notRequired})
	}
	if err != nil {
		return
	}
	// receive connect target
	// version | cmd | reserve | address type
	if len(buffer) < 10 {
		buffer = make([]byte, 4+net.IPv4len+2) // 4 + 4(ipv4) + 2(port)
	}
	_, err = io.ReadAtLeast(c.conn, buffer[:4], 4)
	if err != nil {
		return
	}
	if buffer[0] != version5 {
		c.log(logger.Exploit, "unexpected connect protocol version")
		return
	}
	if buffer[1] != connect {
		c.log(logger.Exploit, "unknown command")
		_, _ = c.conn.Write([]byte{version5, cmdNotSupport, reserve})
		return
	}
	if buffer[2] != reserve { // reserve
		c.log(logger.Exploit, "non-zero reserved field")
		_, err = c.conn.Write([]byte{version5, noReserve, reserve})
		return
	}
	// read address
	var host string
	switch buffer[3] {
	case ipv4:
		_, err = io.ReadAtLeast(c.conn, buffer[:net.IPv4len], net.IPv4len)
		if err != nil {
			return
		}
		host = net.IP(buffer[:net.IPv4len]).String()
	case ipv6:
		buffer = make([]byte, net.IPv6len)
		_, err = io.ReadAtLeast(c.conn, buffer[:net.IPv6len], net.IPv6len)
		if err != nil {
			return
		}
		host = net.IP(buffer[:net.IPv6len]).String()
	case fqdn:
		// get FQDN length
		_, err = io.ReadAtLeast(c.conn, buffer[:1], 1)
		if err != nil {
			return
		}
		l = int(buffer[0])
		if l > len(buffer) {
			buffer = make([]byte, l)
		}
		_, err = io.ReadAtLeast(c.conn, buffer[:l], l)
		if err != nil {
			return
		}
		host = string(buffer[:l])
	default:
		c.log(logger.Exploit, "invalid address type")
		_, _ = c.conn.Write(v5ReplyAddressNotSupport)
		return
	}
	// get port
	_, err = io.ReadAtLeast(c.conn, buffer[:2], 2)
	if err != nil {
		return
	}
	// connect target
	port := convert.BytesToUint16(buffer[:2])
	address := net.JoinHostPort(host, strconv.Itoa(int(port)))
	c.log(logger.Debug, "connect: "+address)
	remote, err := c.server.dialTimeout("tcp", address, c.server.timeout)
	if err != nil {
		_, _ = c.conn.Write(v5ReplyConnectRefused)
		return
	}
	// write reply
	// ipv4 + 0.0.0.0 + 0(port)
	_, err = c.conn.Write(v5ReplySucceeded)
	if err != nil {
		_ = remote.Close()
		return
	}
	c.remote = remote
}

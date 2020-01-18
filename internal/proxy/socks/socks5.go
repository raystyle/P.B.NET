package socks

import (
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
)

// https://www.ietf.org/rfc/rfc1928.txt

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

func (c *Client) connectSocks5(conn net.Conn, host string, port uint16) error {
	// request authentication
	buf := bytes.Buffer{}
	buf.WriteByte(version5)
	if c.username == "" {
		buf.WriteByte(1)
		buf.WriteByte(notRequired)
	} else {
		buf.WriteByte(2)
		buf.WriteByte(notRequired)
		buf.WriteByte(usernamePassword)
	}
	_, err := conn.Write(buf.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to write socks5 request authentication")
	}
	reply := make([]byte, 2)
	_, err = io.ReadFull(conn, reply)
	if err != nil {
		return errors.Wrap(err, "failed to read socks5 request authentication")
	}
	if reply[0] != version5 {
		return errors.Errorf("unexpected socks5 version %d", reply[0])
	}
	err = c.authenticate(reply[1], conn)
	if err != nil {
		return err
	}
	// send connect target
	buf.Reset()
	buf.WriteByte(version5)
	buf.WriteByte(connect)
	buf.WriteByte(reserve)
	ip := net.ParseIP(host)
	if ip != nil {
		ip4 := ip.To4()
		if ip4 != nil {
			buf.WriteByte(ipv4)
			buf.Write(ip4)
		} else {
			buf.WriteByte(ipv6)
			buf.Write(ip.To16())
		}
	} else {
		if len(host) > 255 {
			return errors.New("FQDN too long")
		}
		buf.WriteByte(fqdn)
		buf.WriteByte(byte(len(host)))
		buf.Write([]byte(host))
	}
	buf.Write(convert.Uint16ToBytes(port))
	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to write connect target")
	}
	return c.receiveReply(conn)
}

func (c *Client) authenticate(am uint8, conn net.Conn) error {
	switch am {
	case notRequired:
	case usernamePassword:
		username := c.username
		password := c.password
		if len(username) == 0 || len(username) > 255 {
			return errors.New("invalid username length")
		}
		// https://www.ietf.org/rfc/rfc1929.txt
		buf := bytes.Buffer{}
		buf.WriteByte(usernamePasswordVersion)
		buf.WriteByte(byte(len(username)))
		buf.WriteString(username)
		buf.WriteByte(byte(len(password)))
		buf.WriteString(password)
		_, err := conn.Write(buf.Bytes())
		if err != nil {
			return errors.Wrap(err, "failed to write socks5 username password")
		}
		response := make([]byte, 2)
		_, err = io.ReadFull(conn, response)
		if err != nil {
			return errors.Wrap(err, "failed to read socks5 username password reply")
		}
		if response[0] != usernamePasswordVersion {
			return errors.New("invalid username/password version")
		}
		if response[1] != statusSucceeded {
			return errors.New("invalid username/password")
		}
	case noAcceptableMethods:
		return errors.New("no acceptable authentication methods")
	default:
		return errors.Errorf("unsupported authentication method %d", am)
	}
	return nil
}

func (c *Client) receiveReply(conn net.Conn) error {
	// receive reply
	reply := make([]byte, 4)
	_, err := io.ReadFull(conn, reply)
	if err != nil {
		return errors.Wrap(err, "failed to read connect target reply")
	}
	if reply[0] != version5 {
		return errors.Errorf("unexpected socks5 version %d", reply[0])
	}
	if reply[1] != succeeded {
		return errors.New(v5Reply(reply[1]).String())
	}
	if reply[2] != 0 {
		return errors.New("non-zero reserved field")
	}
	l := 2 // port
	switch reply[3] {
	case ipv4:
		l += net.IPv4len
	case ipv6:
		l += net.IPv6len
	case fqdn:
		_, err = io.ReadFull(conn, reply[:1])
		if err != nil {
			return errors.Wrap(err, "failed to read connect target reply FQDN size")
		}
		l += int(reply[0])
	}
	// grow
	if cap(reply) < l {
		reply = make([]byte, l)
	} else {
		reply = reply[:l]
	}
	_, err = io.ReadFull(conn, reply)
	return errors.Wrap(err, "failed to read socks5 rest reply")
}

var (
	v5ReplySucceeded         = []byte{version5, succeeded, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	v5ReplyConnectRefused    = []byte{version5, connRefused, reserve, ipv4, 0, 0, 0, 0, 0, 0}
	v5ReplyAddressNotSupport = []byte{version5, addrNotSupport, reserve, ipv4, 0, 0, 0, 0, 0, 0}
)

func (c *conn) serveSocks5() {
	buf := make([]byte, 4)
	// read version
	_, err := io.ReadAtLeast(c.local, buf[:1], 1)
	if err != nil {
		c.log(logger.Error, errors.Wrap(err, "failed to read socks5 version"))
		return
	}
	if buf[0] != version5 {
		c.log(logger.Error, "unexpected socks5 version")
		return
	}
	// read authentication methods
	_, err = io.ReadAtLeast(c.local, buf[:1], 1)
	if err != nil {
		const msg = "failed to read the number of the authentication methods"
		c.log(logger.Error, errors.Wrap(err, msg))
		return
	}
	l := int(buf[0])
	if l == 0 {
		c.log(logger.Error, "no authentication method")
		return
	}
	if l > len(buf) {
		buf = make([]byte, l)
	}
	_, err = io.ReadAtLeast(c.local, buf[:l], l)
	if err != nil {
		c.log(logger.Error, errors.Wrap(err, "failed to read authentication methods"))
		return
	}
	if !c.authenticate() {
		return
	}
	target := c.receiveTarget()
	if target == "" {
		return
	}
	// connect target
	c.log(logger.Debug, "connect: "+target)
	ctx, cancel := context.WithTimeout(c.server.ctx, c.server.timeout)
	defer cancel()
	remote, err := c.server.dialContext(ctx, "tcp", target)
	if err != nil {
		c.log(logger.Error, errors.WithStack(err))
		_, _ = c.local.Write(v5ReplyConnectRefused)
		return
	}
	// write reply
	_, err = c.local.Write(v5ReplySucceeded)
	if err != nil {
		c.log(logger.Error, errors.WithStack(err))
		_ = remote.Close()
		return
	}
	c.remote = remote
}

func (c *conn) authenticate() bool {
	var err error
	if c.server.username != nil {
		_, err = c.local.Write([]byte{version5, usernamePassword})
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to write authentication methods"))
			return false
		}
		buf := make([]byte, 16)
		// read username and password version
		_, err = io.ReadAtLeast(c.local, buf[:1], 1)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read username password version"))
			return false
		}
		if buf[0] != usernamePasswordVersion {
			c.log(logger.Error, "unexpected username password version")
			return false
		}
		// read username length
		_, err = io.ReadAtLeast(c.local, buf[:1], 1)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read username length"))
			return false
		}
		l := int(buf[0])
		if l > len(buf) {
			buf = make([]byte, l)
		}
		// read username
		_, err = io.ReadAtLeast(c.local, buf[:l], l)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read username"))
			return false
		}
		username := make([]byte, l)
		copy(username, buf[:l])
		// read password length
		_, err = io.ReadAtLeast(c.local, buf[:1], 1)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read password length"))
			return false
		}
		l = int(buf[0])
		if l > len(buf) {
			buf = make([]byte, l)
		}
		// read password
		_, err = io.ReadAtLeast(c.local, buf[:l], l)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read password"))
			return false
		}
		password := make([]byte, l)
		copy(password, buf[:l])
		// write username password version
		_, err = c.local.Write([]byte{usernamePasswordVersion})
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to write username password version"))
			return false
		}
		if subtle.ConstantTimeCompare(c.server.username, username) != 1 ||
			subtle.ConstantTimeCompare(c.server.password, password) != 1 {
			l := fmt.Sprintf("invalid username password: %s %s", username, password)
			c.log(logger.Exploit, l)
			_, _ = c.local.Write([]byte{statusFailed})
			return false
		}
		_, err = c.local.Write([]byte{statusSucceeded})
	} else {
		_, err = c.local.Write([]byte{version5, notRequired})
	}
	if err != nil {
		c.log(logger.Error, errors.Wrap(err, "failed to write authentication reply"))
		return false
	}
	return true
}

// receiveTarget receive connect target
// version | cmd | reserve | address type
func (c *conn) receiveTarget() string {
	buf := make([]byte, 4+net.IPv4len+2) // 4 + 4(ipv4) + 2(port)
	_, err := io.ReadAtLeast(c.local, buf[:4], 4)
	if err != nil {
		c.log(logger.Error, errors.Wrap(err, "failed to read version cmd address type"))
		return ""
	}
	if buf[0] != version5 {
		c.log(logger.Exploit, "unexpected socks5 version")
		return ""
	}
	if buf[1] != connect {
		c.log(logger.Exploit, "unknown command")
		_, _ = c.local.Write([]byte{version5, cmdNotSupport, reserve})
		return ""
	}
	if buf[2] != reserve { // reserve
		c.log(logger.Exploit, "non-zero reserved field")
		_, _ = c.local.Write([]byte{version5, noReserve, reserve})
		return ""
	}
	// read host
	var host string
	switch buf[3] {
	case ipv4:
		_, err = io.ReadAtLeast(c.local, buf[:net.IPv4len], net.IPv4len)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read IPv4 address"))
			return ""
		}
		host = net.IP(buf[:net.IPv4len]).String()
	case ipv6:
		buf = make([]byte, net.IPv6len)
		_, err = io.ReadAtLeast(c.local, buf[:net.IPv6len], net.IPv6len)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read IPv6 address"))
			return ""
		}
		host = net.IP(buf[:net.IPv6len]).String()
	case fqdn:
		// get FQDN length
		_, err = io.ReadAtLeast(c.local, buf[:1], 1)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read FQDN length"))
			return ""
		}
		l := int(buf[0])
		if l > len(buf) {
			buf = make([]byte, l)
		}
		_, err = io.ReadAtLeast(c.local, buf[:l], l)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read FQDN"))
			return ""
		}
		host = string(buf[:l])
	default:
		c.log(logger.Exploit, "invalid address type")
		_, _ = c.local.Write(v5ReplyAddressNotSupport)
		return ""
	}
	// get port
	_, err = io.ReadAtLeast(c.local, buf[:2], 2)
	if err != nil {
		c.log(logger.Error, errors.Wrap(err, "failed to read port"))
		return ""
	}
	port := convert.BytesToUint16(buf[:2])
	return net.JoinHostPort(host, strconv.Itoa(int(port)))
}

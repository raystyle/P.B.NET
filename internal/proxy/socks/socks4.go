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

// http://ftp.icm.edu.pl/packages/socks/socks4/SOCKS4.protocol
// http://www.openssh.com/txt/socks4a.protocol

const (
	version4 uint8 = 0x04
)

var v4IPPadding = []byte{0x00, 0x00, 0x00, 0x01} // domain

type v4Reply uint8

func (r v4Reply) String() string {
	switch r {
	case 0x5b:
		return "request rejected or failed"
	case 0x5c:
		return "request rejected because SOCKS server cannot connect to ident on the client"
	case 0x5d:
		return "request rejected because the client program and ident report different user-ids"
	default:
		return "unknown reply: " + strconv.Itoa(int(r))
	}
}

func (c *Client) connectSocks4(conn net.Conn, host string, port uint16) error {
	var (
		hostData   []byte
		socks4aExt bool
	)
	ip := net.ParseIP(host)
	if ip != nil {
		ip4 := ip.To4()
		if ip4 != nil {
			hostData = ip4
		} else {
			return errors.New("socks4 or socks4a don't support IPv6")
		}
	} else if c.disableExt {
		return errors.Errorf("%s is not a socks4a server", c.address)
	} else {
		l := len(host)
		if l > 255 {
			return errors.New("hostname too long")
		}
		hostData = make([]byte, l)
		copy(hostData, host)
		socks4aExt = true
	}

	// handshake
	buffer := bytes.Buffer{}
	buffer.WriteByte(version4)
	buffer.WriteByte(connect)
	buffer.Write(convert.Uint16ToBytes(port))
	if socks4aExt { // socks4a ext
		buffer.Write(v4IPPadding) // padding IPv4
	} else {
		buffer.Write(hostData) // IPv4
	}
	// user id
	buffer.Write(c.userID)
	buffer.WriteByte(0x00) // NULL
	// write domain
	if socks4aExt {
		buffer.Write(hostData)
		buffer.WriteByte(0x00) // NULL
	}
	_, err := conn.Write(buffer.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to write socks4 request data")
	}

	// read response, version4, reply, port, ip
	reply := make([]byte, 1+1+2+net.IPv4len)
	_, err = io.ReadFull(conn, reply)
	if err != nil {
		return errors.Wrap(err, "failed to read socks4 reply")
	}
	if reply[0] != 0x00 { // must 0x00 not 0x04
		return errors.Errorf("invalid socks version %d", reply[0])
	}
	if reply[1] != 0x5a {
		return errors.New(v4Reply(reply[1]).String())
	}
	return nil
}

var (
	v4ReplySucceeded      = []byte{0x00, 0x5a, 0, 0, 0, 0, 0, 0}
	v4ReplyConnectRefused = []byte{0x00, 0x5b, 0, 0, 0, 0, 0, 0}
)

func (c *conn) serveSocks4() {
	// 10 = version(1) + cmd(1) + port(2) + address(4) + 2xNULL(2) maybe
	// 16 = domain name
	buffer := make([]byte, 10+16) // prepare
	_, err := io.ReadAtLeast(c.local, buffer[:8], 8)
	if err != nil {
		c.log(logger.Error, errors.Wrap(err, "failed to read socks4 request"))
		return
	}
	// check version
	if buffer[0] != version4 {
		c.log(logger.Error, "unexpected socks4 version")
		return
	}
	// command
	if buffer[1] != connect {
		c.log(logger.Error, "unknown command")
		return
	}
	if !c.checkUserID() {
		return
	}
	// address
	port := convert.BytesToUint16(buffer[2:4])
	var (
		domain bool
		ip     bool
		host   string
	)
	if c.server.disableExt {
		ip = true
	} else {
		// check is domain, 0.0.0.x is domain mode
		if bytes.Compare(buffer[4:7], []byte{0x00, 0x00, 0x00}) == 0 && buffer[7] != 0x00 {
			domain = true
		} else {
			ip = true
		}
	}
	if ip {
		host = net.IPv4(buffer[4], buffer[5], buffer[6], buffer[7]).String()
	}
	if domain { // read domain // TODO disable ext
		var domainName []byte
		for {
			_, err = c.local.Read(buffer[:1])
			if err != nil {
				c.log(logger.Error, errors.Wrap(err, "failed to read domain name"))
				return
			}
			// find 0x00(end)
			if buffer[0] == 0x00 {
				break
			}
			domainName = append(domainName, buffer[0])
		}
		host = string(domainName)
	}
	address := net.JoinHostPort(host, strconv.Itoa(int(port)))
	// connect target
	c.log(logger.Debug, "connect:", address)
	ctx, cancel := context.WithTimeout(c.server.ctx, c.server.timeout)
	defer cancel()
	remote, err := c.server.dialContext(ctx, "tcp", address)
	if err != nil {
		c.log(logger.Error, errors.WithStack(err))
		_, _ = c.local.Write(v4ReplyConnectRefused)
		return
	}
	// write reply
	_, err = c.local.Write(v4ReplySucceeded)
	if err != nil {
		c.log(logger.Error, errors.WithStack(err))
		_ = remote.Close()
		return
	}
	c.remote = remote
}

func (c *conn) checkUserID() bool {
	var (
		userID []byte
		err    error
	)
	buffer := make([]byte, 1)
	for {
		_, err = c.local.Read(buffer)
		if err != nil {
			c.log(logger.Error, errors.Wrap(err, "failed to read user id"))
			return false
		}
		// find 0x00(end)
		if buffer[0] == 0x00 {
			break
		}
		userID = append(userID, buffer[0])
	}
	// compare user id
	if subtle.ConstantTimeCompare(c.server.userID, userID) != 1 {
		c.log(logger.Exploit, fmt.Sprintf("invalid user id: %s", userID))
		return false
	}
	return true
}

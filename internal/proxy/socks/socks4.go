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

const (
	version4 uint8 = 0x04
)

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

// http://ftp.icm.edu.pl/packages/socks/socks4/SOCKS4.protocol
// http://www.openssh.com/txt/socks4a.protocol

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
		buffer.Write([]byte{0x00, 0x00, 0x00, 0x01}) // padding IPv4
	} else {
		buffer.Write(hostData) // IPv4
	}
	// user id
	buffer.Write(c.userID)
	buffer.WriteByte(0x00) // NULL
	// write host
	if socks4aExt {
		buffer.Write(hostData)
		buffer.WriteByte(0x00) // NULL
	}
	_, err := conn.Write(buffer.Bytes())
	if err != nil {
		return errors.WithStack(err)
	}
	// read response
	// version4, reply, port, ip
	resp := make([]byte, 1+1+2+net.IPv4len)
	_, err = io.ReadFull(conn, resp)
	if err != nil {
		return errors.WithStack(err)
	}
	if resp[0] != 0x00 { // must 0x00 not 0x04
		return errors.Errorf("invalid version %d", resp[0])
	}
	if resp[1] != 0x5a {
		return errors.New(v4Reply(resp[1]).String())
	}
	return nil
}

// http://ftp.icm.edu.pl/packages/socks/socks4/SOCKS4.protocol
// http://www.openssh.com/txt/socks4a.protocol

var (
	v4ReplySucceeded      = []byte{0x00, 0x5a, 0, 0, 0, 0, 0, 0}
	v4ReplyConnectRefused = []byte{0x00, 0x5b, 0, 0, 0, 0, 0, 0}
)

func (c *conn) serveSocks4() {
	var err error
	defer func() {
		if err != nil {
			c.log(logger.Error, err)
		}
	}()
	// 10 = version(1) + cmd(1) + port(2) + address(4) + 2xNULL(2) maybe
	// 16 = domain name
	buffer := make([]byte, 10+16) // prepare
	_, err = io.ReadAtLeast(c.local, buffer[:8], 8)
	if err != nil {
		return
	}
	// check version
	if buffer[0] != version4 {
		c.log(logger.Error, "unexpected protocol version")
		return
	}
	// command
	if buffer[1] != connect {
		c.log(logger.Error, "unknown command")
		return
	}
	port := convert.BytesToUint16(buffer[2:4])
	var (
		domain bool
		host   string
	)
	// check is domain 0.0.0.x is domain mode
	if bytes.Equal(buffer[4:7], []byte{0x00, 0x00, 0x00}) && buffer[7] != 0x00 {
		domain = true
	} else {
		host = net.IPv4(buffer[4], buffer[5], buffer[6], buffer[7]).String()
	}
	var userID []byte
	for {
		_, err = c.local.Read(buffer[:1])
		if err != nil {
			return
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
		return
	}
	if domain { // read domain
		var dn []byte
		for {
			_, err = c.local.Read(buffer[:1])
			if err != nil {
				return
			}
			// find 0x00(end)
			if buffer[0] == 0x00 {
				break
			}
			dn = append(dn, buffer[0])
		}
		host = string(dn)
	}
	// connect target
	address := net.JoinHostPort(host, strconv.Itoa(int(port)))
	c.log(logger.Debug, "connect: "+address)
	ctx, cancel := context.WithTimeout(c.server.ctx, c.server.timeout)
	defer cancel()
	remote, err := c.server.dialContext(ctx, "tcp4", address)
	if err != nil {
		_, _ = c.local.Write(v4ReplyConnectRefused)
		return
	}
	// write reply
	_, err = c.local.Write(v4ReplySucceeded)
	if err != nil {
		_ = remote.Close()
		return
	}
	c.remote = remote
}

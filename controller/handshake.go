package controller

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

// handshake error
type hs_err struct {
	c net.Conn
	s string
	e error
}

// "tcp 127.0.0.1:1234 <-> tcp 127.0.0.1:1235 [ver: 1] send data failed: error"
func (this *hs_err) Error() string {
	b := &bytes.Buffer{}
	_, _ = fmt.Fprintf(b, "%s %s <-> %s %s ",
		this.c.LocalAddr().Network(), this.c.LocalAddr(),
		this.c.RemoteAddr().Network(), this.c.RemoteAddr())
	b.WriteString(this.s)
	if this.e != nil {
		b.WriteString(": ")
		b.WriteString(this.e.Error())
	}
	return b.String()
}

// certificates = [2 byte uint16] size + signature with node guid
//                [2 byte uint16] size + signature with ctrl guid
func (this *client) handshake(c net.Conn) (*xnet.Conn, error) {
	conn := xnet.New_Conn(c, this.ctx.global.Now().Unix())
	// receive certificates
	certificates, err := conn.Receive()
	if err != nil {
		e := &hs_err{c: conn, s: "receive certificate failed", e: err}
		return nil, errors.WithStack(e)
	}
	// if guid = nil, skip verify
	if this.guid != nil {
		const (
			act = "read certificate(with node guid) "
		)
		reader := bytes.NewReader(certificates)
		// read cert size
		cert_size := make([]byte, 2)
		_, err = io.ReadFull(reader, cert_size)
		if err != nil {
			e := &hs_err{c: conn, s: act + "size failed", e: err}
			return nil, errors.WithStack(e)
		}
		// read cert
		cert_with_node_guid := make([]byte, convert.Bytes_Uint16(cert_size))
		_, err = io.ReadFull(reader, cert_size)
		if err != nil {
			e := &hs_err{c: conn, s: act + "failed", e: err}
			return nil, errors.WithStack(e)
		}
		// cacl cert
		buffer := new(bytes.Buffer)
		buffer.Write([]byte(this.node.Mode))
		buffer.Write([]byte(this.node.Network))
		buffer.Write([]byte(this.node.Address))
		buffer.Write(this.guid)
		if bytes.Equal(this.guid, protocol.CTRL_GUID) {
			const (
				act = "read certificate(with controller guid) "
			)
			// read cert size
			_, err = io.ReadFull(reader, cert_size)
			if err != nil {
				e := &hs_err{c: conn, s: act + "size failed", e: err}
				return nil, errors.WithStack(e)
			}
			// read cert
			cert_with_ctrl_guid := make([]byte, convert.Bytes_Uint16(cert_size))
			_, err = io.ReadFull(reader, cert_size)
			if err != nil {
				e := &hs_err{c: conn, s: act + "failed", e: err}
				return nil, errors.WithStack(e)
			}
			if !this.ctx.global.Verify(buffer.Bytes(), cert_with_ctrl_guid) {
				e := &hs_err{c: conn, s: "invalid certificate(with controller guid)"}
				this.log(logger.EXPLOIT, e)
				return nil, errors.WithStack(e)
			}
		} else {
			if !this.ctx.global.Verify(buffer.Bytes(), cert_with_node_guid) {
				e := &hs_err{c: conn, s: "invalid certificate(with node guid)"}
				this.log(logger.EXPLOIT, e)
				return nil, errors.WithStack(e)
			}
		}
	}
	// send role
	_, err = conn.Write([]byte{protocol.CTRL})
	if err != nil {
		e := &hs_err{c: conn, s: "send role failed", e: err}
		return nil, errors.WithStack(e)
	}
	err = this.authenticate(conn)
	if err != nil {
		e := &hs_err{c: conn, s: "authenticate failed", e: err}
		this.log(logger.EXPLOIT, e)
		return nil, errors.WithStack(e)
	}
	return conn, nil
}

func (this *client) authenticate(conn *xnet.Conn) error {
	// receive challenge
	challenge, err := conn.Receive()
	if err != nil {
		e := &hs_err{c: conn, s: "receive challenge data failed", e: err}
		return errors.WithStack(e)
	}
	// <danger>
	// receive random challenge data(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and if controller sign it will destory net
	if len(challenge) < 2048 || len(challenge) > 4096 {
		e := &hs_err{c: conn, s: "invalid challenge size"}
		this.log(logger.EXPLOIT, e)
		return errors.WithStack(e)
	}
	// send signature
	err = conn.Send(this.ctx.global.Sign(challenge))
	if err != nil {
		e := &hs_err{c: conn, s: "send challenge signature failed", e: err}
		return errors.WithStack(e)
	}
	resp, err := conn.Receive()
	if err != nil {
		e := &hs_err{c: conn, s: "receive authentication response failed", e: err}
		return errors.WithStack(e)
	}
	if !bytes.Equal(resp, protocol.AUTH_SUCCESS) {
		e := &hs_err{c: conn, e: protocol.ERR_AUTH_FAILED}
		return errors.WithStack(e)
	}
	return nil
}

// TODO log print

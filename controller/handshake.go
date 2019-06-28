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

// handshake log
type hs_err struct {
	c net.Conn
	s string
	e error
}

// "tcp 127.0.0.1:1234 <-> tcp 127.0.0.1:1235 ver: 1 send data failed: error"
func (this *hs_err) Error() string {
	b := bytes.Buffer{}
	b.WriteString(fmt.Sprintf("%s %s <-> %s %s ",
		this.c.LocalAddr().Network(), this.c.LocalAddr(),
		this.c.RemoteAddr().Network(), this.c.RemoteAddr()))
	if conn, ok := this.c.(*xnet.Conn); ok {
		b.WriteString(fmt.Sprintf("ver: %d ", conn.Info().Version))
	}
	b.WriteString(this.s)
	if this.e != nil {
		b.WriteString(": ")
		b.WriteString(this.e.Error())
	}
	return b.String()
}

func (this *client) handshake(conn net.Conn, skip_verify bool) (*xnet.Conn, error) {
	// receive node support version & select used version
	buffer := make([]byte, 4)
	_, err := io.ReadFull(conn, buffer)
	if err != nil {
		e := &hs_err{c: conn, s: "receive version failed", e: err}
		return nil, errors.WithStack(e)
	}
	ver := convert.Bytes_Uint32(buffer)
	if ver >= version {
		ver = version
	}
	// send used version
	_, err = conn.Write(convert.Uint32_Bytes(ver))
	if err != nil {
		e := &hs_err{c: conn, s: "send version failed", e: err}
		return nil, errors.WithStack(e)
	}
	c := xnet.New_Conn(conn, this.ctx.global.Now().Unix(), ver)
	// verify
	switch {
	case ver == protocol.V1_0_0:
		err = this.v1_verify(c, skip_verify)
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (this *client) v1_verify(conn *xnet.Conn, skip_verify bool) error {
	// receive certificate
	cert, err := conn.Receive()
	if err != nil {
		e := &hs_err{c: conn, s: "receive certificate failed", e: err}
		return errors.WithStack(e)
	}
	if !skip_verify {
		cert[0] = 100
	}
	// send role
	_, err = conn.Write([]byte{protocol.CTRL})
	if err != nil {
		e := &hs_err{c: conn, s: "send role failed", e: err}
		return errors.WithStack(e)
	}
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
		this.ctx.Println(logger.EXPLOIT, "client", e)
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
		e := &hs_err{c: conn, s: "receive authorization response failed", e: err}
		return errors.WithStack(e)
	}
	if !bytes.Equal(resp, protocol.AUTH_SUCCESS) {
		e := &hs_err{c: conn, s: "authorization failed", e: err}
		return errors.WithStack(e)
	}
	return nil
}

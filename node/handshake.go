package node

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
	"project/internal/xpanic"
)

// serve log(handshake client)
type s_log struct {
	c net.Conn
	l string
	e error
}

func (this *s_log) String() string {
	b := &bytes.Buffer{}
	_, _ = fmt.Fprintf(b, "%s %s <-> %s %s ",
		this.c.LocalAddr().Network(), this.c.LocalAddr(),
		this.c.RemoteAddr().Network(), this.c.RemoteAddr())
	b.WriteString(this.l)
	if this.e != nil {
		b.WriteString(": ")
		b.WriteString(this.e.Error())
	}
	return b.String()
}

func (this *server) handshake(l_tag string, conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("handshake panic:", r)
			this.log(logger.EXPLOIT, &s_log{c: conn, l: "", e: err})
		}
		_ = conn.Close()
	}()
	// add to conns for management
	xconn := xnet.New_Conn(conn, this.ctx.global.Now().Unix())
	// conn tag
	b := bytes.Buffer{}
	b.WriteString(l_tag)
	b.WriteString(conn.RemoteAddr().String())
	conn_tag := b.String()
	this.add_conn(conn_tag, xconn)
	defer this.del_conn(conn_tag)
	// send certificate
	var err error
	cert := this.ctx.global.Cert()
	if cert != nil {
		err = xconn.Send(cert)
		if err != nil {
			l := &s_log{c: conn, l: "send certificate failed", e: err}
			this.log(logger.ERROR, l)
			return
		}
	} else { // if no certificate send padding data
		padding_size := 1024 + this.random.Int(1024)
		err = xconn.Send(this.random.Bytes(padding_size))
		if err != nil {
			l := &s_log{c: conn, l: "send padding data failed", e: err}
			this.log(logger.ERROR, l)
			return
		}
	}
	// receive role
	role := make([]byte, 1)
	_, err = io.ReadFull(conn, role)
	if err != nil {
		l := &s_log{c: conn, l: "receive role failed", e: err}
		this.log(logger.ERROR, l)
		return
	}
	switch role[0] {
	case protocol.BEACON:
		this.verify_beacon(xconn)
	case protocol.NODE:
		this.verify_node(xconn)
	case protocol.CTRL:
		this.verify_ctrl(xconn)
	default:
		this.log(logger.EXPLOIT, &s_log{c: conn, e: protocol.ERR_INVALID_ROLE})
	}
}

func (this *server) verify_beacon(conn *xnet.Conn) {

}

func (this *server) verify_node(conn *xnet.Conn) {

}

func (this *server) verify_ctrl(conn *xnet.Conn) {
	// <danger>
	// send random challenge code(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and controller sign it
	challenge := this.random.Bytes(2048 + this.random.Int(2048))
	err := conn.Send(challenge)
	if err != nil {
		l := &s_log{c: conn, l: "send challenge code failed", e: err}
		this.log(logger.ERROR, l)
		return
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		l := &s_log{c: conn, l: "receive signature failed", e: err}
		this.log(logger.ERROR, l)
		return
	}
	// verify signature
	if !this.ctx.global.CTRL_Verify(challenge, signature) {
		l := &s_log{c: conn, l: "invalid controller signature", e: err}
		this.log(logger.EXPLOIT, l)
		return
	}
	// send success
	err = conn.Send(protocol.AUTH_SUCCESS)
	if err != nil {
		l := &s_log{c: conn, l: "send auth success response failed", e: err}
		this.log(logger.ERROR, l)
		return
	}
	this.serve_ctrl(conn)
}

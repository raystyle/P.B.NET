package node

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

// handshake log
type hs_log struct {
	c net.Conn
	l string
	e error
}

func (this *hs_log) String() string {
	b := bytes.Buffer{}
	b.WriteString(fmt.Sprintf("%s %s <-> %s %s ",
		this.c.LocalAddr().Network(), this.c.LocalAddr(),
		this.c.RemoteAddr().Network(), this.c.RemoteAddr()))
	if conn, ok := this.c.(*xnet.Conn); ok {
		b.WriteString(fmt.Sprintf("[ver: %d] ", conn.Info().Version))
	}
	b.WriteString(this.l)
	if this.e != nil {
		b.WriteString(": ")
		b.WriteString(this.e.Error())
	}
	return b.String()
}

func (this *server) handshake(l_tag string, conn net.Conn) {
	// add to conns for management
	now := this.ctx.global.Now().Unix()
	c := xnet.New_Conn(conn, now, 0)
	// conn tag
	b := bytes.Buffer{}
	b.WriteString(l_tag)
	b.WriteString(conn.RemoteAddr().String())
	conn_tag := b.String()
	this.add_conn(conn_tag, c)
	defer func() {
		_ = conn.Close()
		this.del_conn(conn_tag)
	}()
	// send support max version
	_, err := conn.Write(convert.Uint32_Bytes(Version))
	if err != nil {
		l := &hs_log{c: conn, l: "send supported version failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	// receive client version
	version := make([]byte, 4)
	_, err = io.ReadFull(conn, version)
	if err != nil {
		l := &hs_log{c: conn, l: "receive version failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	v := convert.Bytes_Uint32(version)
	// cover conn with version
	c = xnet.New_Conn(conn, now, v)
	this.add_conn(conn_tag, c)
	switch {
	case v == protocol.V1_0_0:
		this.v1_authenticate(c)
	default:
		l := &hs_log{c: c, l: fmt.Sprint("unsupport version", v)}
		this.logln(logger.ERROR, l)
	}
}

func (this *server) v1_authenticate(conn *xnet.Conn) {
	// send certificate
	var err error
	cert := this.ctx.global.Cert()
	if cert != nil {
		err = conn.Send(cert)
		if err != nil {
			l := &hs_log{c: conn, l: "send certificate failed", e: err}
			this.logln(logger.ERROR, l)
			return
		}
	} else { // if no certificate send padding data
		padding_size := 1024 + this.random.Int(1024)
		err = conn.Send(this.random.Bytes(padding_size))
		if err != nil {
			l := &hs_log{c: conn, l: "send padding data failed", e: err}
			this.logln(logger.ERROR, l)
			return
		}
	}
	// receive role
	role := make([]byte, 1)
	_, err = io.ReadFull(conn, role)
	if err != nil {
		l := &hs_log{c: conn, l: "receive role failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	switch role[0] {
	case protocol.BEACON:
		this.v1_verify_beacon(conn)
	case protocol.NODE:
		this.v1_verify_node(conn)
	case protocol.CTRL:
		this.v1_verify_ctrl(conn)
	default:
		this.logln(logger.EXPLOIT, &hs_log{c: conn, l: "invalid role"})
	}
}

func (this *server) v1_verify_beacon(conn *xnet.Conn) {

}

func (this *server) v1_verify_node(conn *xnet.Conn) {

}

func (this *server) v1_verify_ctrl(conn *xnet.Conn) {
	// <danger>
	// send random challenge code(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and controller sign it
	challenge := this.random.Bytes(2048 + this.random.Int(2048))
	err := conn.Send(challenge)
	if err != nil {
		l := &hs_log{c: conn, l: "send challenge code failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		l := &hs_log{c: conn, l: "receive signature failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	// verify signature
	if !this.ctx.global.CTRL_Verify(challenge, signature) {
		l := &hs_log{c: conn, l: "invalid controller signature", e: err}
		this.logln(logger.EXPLOIT, l)
		return
	}
	// send success
	err = conn.Send(protocol.AUTH_SUCCESS)
	if err != nil {
		l := &hs_log{c: conn, l: "send auth success response failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	this.logln(logger.INFO, &hs_log{c: conn, l: "new controller connect"})
	this.handle_ctrl(conn)
}

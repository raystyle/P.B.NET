package node

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
)

// handshake log
type hs_log struct {
	c *conn
	l string
	e error
}

func (this *hs_log) String() string {
	b := bytes.Buffer{}
	b.WriteString(fmt.Sprintf("%s %s <-> %s %s ",
		this.c.l_network, this.c.l_address,
		this.c.r_network, this.c.r_address))
	if this.c.version != 0 {
		b.WriteString(fmt.Sprintf("ver: %d ", this.c.version))
	}
	b.WriteString(this.l)
	if this.e != nil {
		b.WriteString(": ")
		b.WriteString(this.e.Error())
	}
	return b.String()
}

func (this *server) handshake(raw net.Conn) {
	conn := &conn{
		Conn:      raw,
		connect:   this.ctx.global.Now().Unix(),
		l_network: raw.LocalAddr().Network(),
		l_address: raw.LocalAddr().String(),
		r_network: raw.RemoteAddr().Network(),
		r_address: raw.RemoteAddr().String(),
	}
	// tag
	b := bytes.Buffer{}
	b.WriteString(conn.l_network)
	b.WriteString(conn.l_address)
	b.WriteString(conn.r_network)
	b.WriteString(conn.r_address)
	tag := b.String()
	err := this.track_conn(tag, conn, true)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
		_ = this.track_conn(tag, conn, false)
	}()
	// send support max version
	_, err = conn.Write(convert.Uint32_Bytes(Version))
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
	conn.version = v
	switch {
	case v == protocol.V1_0_0:
		this.v1_identity(conn)
	default:
		l := &hs_log{c: conn, l: fmt.Sprint("unsupport version", v)}
		this.logln(logger.ERROR, l)
		return
	}
}

func (this *server) v1_identity(conn *conn) {
	// send certificate
	var err error
	cert := this.ctx.global.Cert()
	if cert != nil {
		err = conn.send_msg(cert)
		if err != nil {
			l := &hs_log{c: conn, l: "send certificate failed", e: err}
			this.logln(logger.ERROR, l)
			return
		}
	} else { // if no certificate send padding data
		padding_size := 1024 + this.random.Int(1024)
		err = conn.send_msg(this.random.Bytes(padding_size))
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
		this.v1_handshake_beacon(conn)
	case protocol.NODE:
		this.v1_handshake_node(conn)
	case protocol.CTRL:
		this.v1_handshake_ctrl(conn)
	default:
		this.logln(logger.EXPLOIT, &hs_log{c: conn, l: "invalid role"})
	}
}

func (this *server) v1_handshake_beacon(conn *conn) {

}

func (this *server) v1_handshake_node(conn *conn) {

}

func (this *server) v1_handshake_ctrl(conn *conn) {
	// send random challenge code(length 2048-4096)
	// <danger>
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and controller sign it
	challenge := this.random.Bytes(2048 + this.random.Int(2048))
	err := conn.send_msg(challenge)
	if err != nil {
		l := &hs_log{c: conn, l: "send challenge code failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	// receive signature
	signature, err := conn.recv_msg()
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
	err = conn.send_msg(protocol.AUTH_SUCCESS)
	if err != nil {
		l := &hs_log{c: conn, l: "send auth success response failed", e: err}
		this.logln(logger.ERROR, l)
		return
	}
	this.logln(logger.INFO, &hs_log{c: conn, l: "new controller connect"})
	// handle controller
	// controller.Add(conn)
}

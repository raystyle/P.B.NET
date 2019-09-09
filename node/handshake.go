package node

import (
	"bytes"
	"io"
	"net"

	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
	"project/internal/xpanic"
)

// serve log(handshake client)
type sLog struct {
	c net.Conn
	l string
	e error
}

func (sl *sLog) String() string {
	b := logger.Conn(sl.c)
	b.WriteString(sl.l)
	if sl.e != nil {
		b.WriteString(": ")
		b.WriteString(sl.e.Error())
	}
	return b.String()
}

func (server *server) handshake(lTag string, conn net.Conn) {
	dConn := xnet.NewDeadlineConn(conn, server.hsTimeout)
	xconn := xnet.NewConn(dConn, server.ctx.global.Now().Unix())
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("handshake panic:", r)
			server.log(logger.EXPLOIT, &sLog{c: xconn, e: err})
		}
		_ = xconn.Close()
		server.wg.Done()
	}()
	// conn tag
	b := bytes.Buffer{}
	b.WriteString(lTag)
	b.WriteString(xconn.RemoteAddr().String())
	connTag := b.String()
	// add to conns for management
	server.addConn(connTag, xconn)
	defer server.delConn(connTag)
	// send certificate
	var err error
	cert := server.ctx.global.Certificate()
	if cert != nil {
		err = xconn.Send(cert)
		if err != nil {
			l := &sLog{c: xconn, l: "send certificate failed", e: err}
			server.log(logger.ERROR, l)
			return
		}
	} else { // if no certificate send padding data
		paddingSize := 1024 + server.random.Int(1024)
		err = xconn.Send(server.random.Bytes(paddingSize))
		if err != nil {
			l := &sLog{c: xconn, l: "send padding data failed", e: err}
			server.log(logger.ERROR, l)
			return
		}
	}
	// receive role
	role := make([]byte, 1)
	_, err = io.ReadFull(xconn, role)
	if err != nil {
		l := &sLog{c: xconn, l: "receive role failed", e: err}
		server.log(logger.ERROR, l)
		return
	}
	// remove deadline conn
	xconn = xnet.NewConn(conn, server.ctx.global.Now().Unix())
	switch role[0] {
	case protocol.Beacon:
		server.verifyBeacon(xconn)
	case protocol.Node:
		server.verifyNode(xconn)
	case protocol.Ctrl:
		server.verifyCtrl(xconn)
	default:
		server.log(logger.EXPLOIT, &sLog{c: xconn, e: protocol.ErrInvalidRole})
	}
}

func (server *server) verifyBeacon(conn *xnet.Conn) {

}

func (server *server) verifyNode(conn *xnet.Conn) {

}

func (server *server) verifyCtrl(conn *xnet.Conn) {
	dConn := xnet.NewDeadlineConn(conn, server.hsTimeout)
	xconn := xnet.NewConn(dConn, server.ctx.global.Now().Unix())
	// <danger>
	// send random challenge code(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and controller sign it
	challenge := server.random.Bytes(2048 + server.random.Int(2048))
	err := xconn.Send(challenge)
	if err != nil {
		l := &sLog{c: xconn, l: "send challenge code failed", e: err}
		server.log(logger.ERROR, l)
		return
	}
	// receive signature
	signature, err := xconn.Receive()
	if err != nil {
		l := &sLog{c: xconn, l: "receive signature failed", e: err}
		server.log(logger.ERROR, l)
		return
	}
	// verify signature
	if !server.ctx.global.CTRLVerify(challenge, signature) {
		l := &sLog{c: xconn, l: "invalid controller signature", e: err}
		server.log(logger.EXPLOIT, l)
		return
	}
	// send success
	err = xconn.Send(protocol.AuthSucceed)
	if err != nil {
		l := &sLog{c: xconn, l: "send auth success response failed", e: err}
		server.log(logger.ERROR, l)
		return
	}
	server.serveCtrl(conn)
}

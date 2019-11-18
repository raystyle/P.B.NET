package node

import (
	"net"

	"project/internal/logger"
	"project/internal/protocol"
)

func (conn *conn) onFrameServeNode(frame []byte) {
	if !conn.onFrame(frame) {
		return
	}
	// check command
	switch frame[0] {

	default:
		conn.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		conn.Close()
	}
}

type nodeConn struct {
}

func (server *server) serveNode(conn net.Conn) {

}

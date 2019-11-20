package node

import (
	"net"

	"project/internal/logger"
	"project/internal/protocol"
)

func (c *conn) onFrameServeNode(frame []byte) {
	if !c.onFrame(frame) {
		return
	}
	// check command
	switch frame[0] {

	default:
		c.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		c.Close()
	}
}

type nodeConn struct {
}

func (s *server) serveNode(conn net.Conn) {

}

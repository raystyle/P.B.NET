package node

import (
	"net"

	"project/internal/logger"
	"project/internal/protocol"
)

func (conn *conn) onFrameServeBeacon(frame []byte) {
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

type beaconConn struct {
}

func (server *server) serveBeacon(conn net.Conn) {

}

package node

import (
	"project/internal/logger"
	"project/internal/protocol"
)

func (c *conn) onFrameServeBeacon(frame []byte) {
	if c.onFrame(frame) {
		return
	}
	// check command
	switch frame[0] {

	default:
		c.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		c.Close()
	}
}

type beaconConn struct {
}

func (s *server) serveBeacon(tag string, beaconGUID []byte, conn *conn) {

}

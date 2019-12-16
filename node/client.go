package node

import (
	"bytes"
	"time"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
)

func (c *conn) onFrameClient(frame []byte) {
	if c.onFrame(frame) {
		return
	}
	// check command
	switch frame[0] {
	case protocol.ConnReplyHeartbeat:
		c.heartbeat <- struct{}{}
	default:
		c.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		c.Close()
	}
}

func (c *conn) sendHeartbeat() {
	var err error
	rand := random.New()
	buffer := bytes.NewBuffer(nil)
	for {
		t := time.Duration(30+rand.Int(60)) * time.Second
		select {
		case <-time.After(t):
			// <security> fake traffic
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			fakeSize := 64 + rand.Int(256)
			buffer.Reset()
			buffer.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
			buffer.WriteByte(protocol.ConnSendHeartbeat)
			buffer.Write(rand.Bytes(fakeSize))
			// send
			_ = c.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err = c.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			select {
			case <-c.heartbeat:
			case <-time.After(t):
				c.log(logger.Warning, "receive heartbeat reply timeout")
				_ = c.conn.Close()
				return
			case <-c.stopSignal:
				return
			}
		case <-c.stopSignal:
			return
		}
	}
}

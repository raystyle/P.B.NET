package node

import (
	"bytes"
	"time"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
)

func (conn *conn) onFrameClient(frame []byte) {
	if !conn.onFrame(frame) {
		return
	}
	// check command
	switch frame[0] {
	case protocol.ConnReplyHeartbeat:
		conn.heartbeat <- struct{}{}
	default:
		conn.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		conn.Close()
	}
}

func (conn *conn) sendHeartbeat() {
	defer conn.wg.Done()
	var err error
	rand := random.New(0)
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
			_ = conn.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err = conn.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			select {
			case <-conn.heartbeat:
			case <-time.After(t):
				conn.log(logger.Warning, "receive heartbeat reply timeout")
				_ = conn.conn.Close()
				return
			case <-conn.stopSignal:
				return
			}
		case <-conn.stopSignal:
			return
		}
	}
}

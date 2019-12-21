package node

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
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

type client struct {
	ctx *Node

	node      *bootstrap.Node
	guid      []byte // node guid
	closeFunc func()

	conn      *xnet.Conn
	slots     []*protocol.Slot
	heartbeat chan struct{}
	inSync    int32

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func (client *client) log(l logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprint(b, log...)
	_, _ = fmt.Fprint(b, "\n", client.conn)
	client.ctx.logger.Print(l, "client", b)
}

func (client *client) logf(l logger.Level, format string, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, format, log...)
	_, _ = fmt.Fprint(b, "\n", client.conn)
	client.ctx.logger.Print(l, "client", b)
}

func (client *client) sendHeartbeatLoop() {
	defer client.wg.Done()
	var err error
	r := random.New()
	buffer := bytes.NewBuffer(nil)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		timer.Reset(time.Duration(30+r.Int(60)) * time.Second)
		select {
		case <-timer.C:
			// <security> fake traffic like client
			fakeSize := 64 + r.Int(256)
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			buffer.Reset()
			buffer.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
			buffer.WriteByte(protocol.ConnSendHeartbeat)
			buffer.Write(r.Bytes(fakeSize))
			// send
			_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err = client.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			// receive reply
			timer.Reset(time.Duration(30+r.Int(60)) * time.Second)
			select {
			case <-client.heartbeat:
			case <-timer.C:
				client.log(logger.Warning, "receive heartbeat timeout")
				_ = client.conn.Close()
				return
			case <-client.stopSignal:
				return
			}
		case <-client.stopSignal:
			return
		}
	}
}

func save() {
	// switch certificate
	// if bytes.Equal(guid, protocol.CtrlGUID) {
	//	certWithCtrlGUID := cert[ed25519.SignatureSize:]
	// 	return ctrl.global.Verify(buffer.Bytes(), certWithCtrlGUID)
	// }
	// // ----------------------with node guid--------------------------
	//	// no size
	//	require.False(t, ctrl.verifyCertificate(nil, address, g))
	//	// invalid size
	//	cert := []byte{0, 1}
	//	require.False(t, ctrl.verifyCertificate(cert, address, g))
	//	// invalid certificate
	//	cert = []byte{0, 1, 0}
	//	require.False(t, ctrl.verifyCertificate(cert, address, g))
	//	// -------------------with controller guid-----------------------
	//	// no size
	//	cert = []byte{0, 1, 0}
	//	require.False(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
	//	// invalid size
	//	cert = []byte{0, 1, 0, 0, 1}
	//	require.False(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
	//	// invalid certificate
	//	cert = []byte{0, 1, 0, 0, 1, 0}
	//	require.False(t, ctrl.verifyCertificate(cert, address, protocol.CtrlGUID))
}

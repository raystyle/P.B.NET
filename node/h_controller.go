package node

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
)

type roleCtrl struct {
	ctx        *NODE
	conn       *xnet.Conn
	slots      []*protocol.Slot
	replyTimer *time.Timer
	rand       *random.Rand // for handleHeartbeat()
	buffer     bytes.Buffer // for handleHeartbeat()
	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func (server *server) serveCtrl(conn *xnet.Conn) {
	ctrl := &roleCtrl{
		ctx:        server.ctx,
		conn:       conn,
		slots:      make([]*protocol.Slot, protocol.SlotSize),
		replyTimer: time.NewTimer(time.Second),
		rand:       random.New(server.ctx.global.Now().Unix()),
		stopSignal: make(chan struct{}),
	}
	server.addCtrl(ctrl)
	server.log(logger.DEBUG, &sLog{c: conn, l: "controller connected"})
	defer func() {
		ctrl.Close()
		server.delCtrl("", ctrl)
		server.log(logger.DEBUG, &sLog{c: conn, l: "controller disconnected"})
	}()
	// init slot
	ctrl.slots = make([]*protocol.Slot, protocol.SlotSize)
	for i := 0; i < protocol.SlotSize; i++ {
		s := &protocol.Slot{
			Available: make(chan struct{}, 1),
			Reply:     make(chan []byte, 1),
			Timer:     time.NewTimer(protocol.RecvTimeout),
		}
		s.Available <- struct{}{}
		ctrl.slots[i] = s
	}
	protocol.HandleConn(conn, ctrl.handleMessage)
}

func (ctrl *roleCtrl) Info() *xnet.Info {
	return ctrl.conn.Info()
}

func (ctrl *roleCtrl) Close() {
	ctrl.closeOnce.Do(func() {
		atomic.StoreInt32(&ctrl.inClose, 1)
		close(ctrl.stopSignal)
		_ = ctrl.conn.Close()
		ctrl.wg.Wait()
	})
}

func (ctrl *roleCtrl) isClosed() bool {
	return atomic.LoadInt32(&ctrl.inClose) != 0
}

func (ctrl *roleCtrl) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(ctrl.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	ctrl.ctx.Print(l, "r_ctrl", b)
}

func (ctrl *roleCtrl) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(ctrl.conn)
	_, _ = fmt.Fprint(b, log...)
	ctrl.ctx.Print(l, "r_ctrl", b)
}

func (ctrl *roleCtrl) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(ctrl.conn)
	_, _ = fmt.Fprintln(b, log...)
	ctrl.ctx.Print(l, "r_ctrl", b)
}

// if need async handle message must copy msg first
func (ctrl *roleCtrl) handleMessage(msg []byte) {
	if ctrl.isClosed() {
		return
	}
	if len(msg) < 3 { // cmd(1) + msg id(2) or reply
		ctrl.log(logger.EXPLOIT, protocol.ErrInvalidMsgSize)
		ctrl.Close()
		return
	}
	switch msg[0] {
	case protocol.CtrlReply:
		ctrl.handleReply(msg[1:])
	case protocol.CtrlHeartbeat:
		ctrl.handleHeartbeat()
	case protocol.CtrlTrustNode:
		ctrl.handleTrustNode(msg[1:3])
	case protocol.CtrlTrustNodeData:
		ctrl.handleTrustNodeData(msg[1:3], msg[3:])
	case protocol.ErrNullMsg:
		ctrl.log(logger.EXPLOIT, protocol.ErrRecvNullMsg)
		ctrl.Close()
	case protocol.ErrTooBigMsg:
		ctrl.log(logger.EXPLOIT, protocol.ErrRecvTooBigMsg)
		ctrl.Close()
	case protocol.TestMessage:
		ctrl.reply(msg[1:3], msg[3:])
	default:
		ctrl.log(logger.EXPLOIT, protocol.ErrRecvUnknownCMD, msg[1:])
		ctrl.Close()
	}
}

func (ctrl *roleCtrl) handleHeartbeat() {
	// <security> fake flow like client
	fakeSize := 64 + ctrl.rand.Int(256)
	// size(4 Bytes) + heartbeat(1 byte) + fake data
	ctrl.buffer.Reset()
	ctrl.buffer.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
	ctrl.buffer.WriteByte(protocol.NodeHeartbeat)
	ctrl.buffer.Write(ctrl.rand.Bytes(fakeSize))
	// send
	_ = ctrl.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = ctrl.conn.Write(ctrl.buffer.Bytes())
}

func (ctrl *roleCtrl) reply(id, reply []byte) {
	if ctrl.isClosed() {
		return
	}
	l := len(reply)
	// 7 = size(4 Bytes) + NodeReply(1 byte) + msg id(2 bytes)
	b := make([]byte, 7+l)
	copy(b, convert.Uint32ToBytes(uint32(3+l))) // write size
	b[4] = protocol.NodeReply
	copy(b[5:7], id)
	copy(b[7:], reply)
	_ = ctrl.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = ctrl.conn.Write(b)
}

// msg_id(2 bytes) + data
func (ctrl *roleCtrl) handleReply(reply []byte) {
	l := len(reply)
	if l < 2 {
		ctrl.log(logger.EXPLOIT, protocol.ErrRecvInvalidMsgIDSize)
		ctrl.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:2]))
	if id > protocol.MaxMsgID {
		ctrl.log(logger.EXPLOIT, protocol.ErrRecvInvalidMsgID)
		ctrl.Close()
		return
	}
	// must copy
	r := make([]byte, l-2)
	copy(r, reply[2:])
	// <security> maybe wrong msg id
	ctrl.replyTimer.Reset(time.Second)
	select {
	case ctrl.slots[id].Reply <- r:
		ctrl.replyTimer.Stop()
	case <-ctrl.replyTimer.C:
		ctrl.log(logger.EXPLOIT, protocol.ErrRecvInvalidReply)
		ctrl.Close()
	}
}

// send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (ctrl *roleCtrl) Send(cmd uint8, data []byte) ([]byte, error) {
	if ctrl.isClosed() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-ctrl.slots[id].Available:
				l := len(data)
				b := make([]byte, 7+l)
				copy(b, convert.Uint32ToBytes(uint32(3+l))) // write size
				b[4] = cmd
				copy(b[5:7], convert.Uint16ToBytes(uint16(id)))
				copy(b[7:], data)
				// send
				_ = ctrl.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := ctrl.conn.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				ctrl.slots[id].Timer.Reset(protocol.RecvTimeout)
				select {
				case r := <-ctrl.slots[id].Reply:
					ctrl.slots[id].Timer.Stop()
					ctrl.slots[id].Available <- struct{}{}
					return r, nil
				case <-ctrl.slots[id].Timer.C:
					ctrl.Close()
					return nil, protocol.ErrRecvTimeout
				case <-ctrl.stopSignal:
					return nil, protocol.ErrConnClosed
				}
			case <-ctrl.stopSignal:
				return nil, protocol.ErrConnClosed
			default:
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-ctrl.stopSignal:
			return nil, protocol.ErrConnClosed
		}
	}
}

func (ctrl *roleCtrl) handleTrustNode(id []byte) {
	ctrl.reply(id, ctrl.ctx.PackOnlineRequest())
}

func (ctrl *roleCtrl) handleTrustNodeData(id []byte, data []byte) {

}

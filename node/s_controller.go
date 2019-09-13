package node

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
	"project/internal/xpanic"
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
	ctrl.log(logger.Debug, "controller connected")
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("serve controller panic:", r)
			ctrl.log(logger.Exploit, err)
		}
		ctrl.Close()
		server.delCtrl("", ctrl)
		ctrl.log(logger.Debug, "controller disconnected")
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
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if ctrl.isClosed() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(msg) < id {
		ctrl.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		ctrl.Close()
		return
	}
	switch msg[0] {
	case protocol.CtrlReply:
		ctrl.handleReply(msg[cmd:])
	case protocol.CtrlHeartbeat:
		ctrl.handleHeartbeat()
	case protocol.CtrlSyncStart:
		ctrl.reply(msg[cmd:id], []byte{protocol.CtrlSyncStart})
	case protocol.CtrlTrustNode:
		ctrl.handleTrustNode(msg[cmd:id])
	case protocol.CtrlTrustNodeData:
		ctrl.handleTrustNodeData(msg[cmd:id], msg[id:])
	case protocol.ErrNullMsg:
		ctrl.log(logger.Exploit, protocol.ErrRecvNullMsg)
		ctrl.Close()
	case protocol.ErrTooBigMsg:
		ctrl.log(logger.Exploit, protocol.ErrRecvTooBigMsg)
		ctrl.Close()
	case protocol.TestMessage:
		ctrl.reply(msg[cmd:id], msg[id:])
	default:
		ctrl.logln(logger.Exploit, protocol.ErrRecvUnknownCMD, msg)
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
	b := make([]byte, protocol.MsgHeaderSize+l)
	// write size
	msgSize := protocol.MsgCMDSize + protocol.MsgIDSize + l
	copy(b, convert.Uint32ToBytes(uint32(msgSize)))
	// write cmd
	b[protocol.MsgLenSize] = protocol.NodeReply
	// write msg id
	copy(b[protocol.MsgLenSize+1:protocol.MsgLenSize+1+protocol.MsgIDSize], id)
	// write data
	copy(b[protocol.MsgHeaderSize:], reply)
	_ = ctrl.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = ctrl.conn.Write(b)
}

// msg id(2 bytes) + data
func (ctrl *roleCtrl) handleReply(reply []byte) {
	l := len(reply)
	if l < protocol.MsgIDSize {
		ctrl.log(logger.Exploit, protocol.ErrRecvInvalidMsgIDSize)
		ctrl.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:protocol.MsgIDSize]))
	if id > protocol.MaxMsgID {
		ctrl.log(logger.Exploit, protocol.ErrRecvInvalidMsgID)
		ctrl.Close()
		return
	}
	// must copy
	r := make([]byte, l-protocol.MsgIDSize)
	copy(r, reply[protocol.MsgIDSize:])
	// <security> maybe wrong msg id
	ctrl.replyTimer.Reset(time.Second)
	select {
	case ctrl.slots[id].Reply <- r:
		ctrl.replyTimer.Stop()
	case <-ctrl.replyTimer.C:
		ctrl.log(logger.Exploit, protocol.ErrRecvInvalidReply)
		ctrl.Close()
	}
}

// Send is use to send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg id(2 bytes) + data
func (ctrl *roleCtrl) Send(cmd uint8, data []byte) ([]byte, error) {
	if ctrl.isClosed() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-ctrl.slots[id].Available:
				l := len(data)
				b := make([]byte, protocol.MsgHeaderSize+l)
				// write MsgLen
				msgSize := protocol.MsgCMDSize + protocol.MsgIDSize + l
				copy(b, convert.Uint32ToBytes(uint32(msgSize)))
				// write cmd
				b[protocol.MsgLenSize] = cmd
				// write msg id
				copy(b[protocol.MsgLenSize+1:protocol.MsgLenSize+1+protocol.MsgIDSize],
					convert.Uint16ToBytes(uint16(id)))
				// write data
				copy(b[protocol.MsgHeaderSize:], data)
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
	// must copy
	cert := make([]byte, len(data))
	copy(cert, data)
	err := ctrl.ctx.global.SetCertificate(cert)
	if err == nil {
		ctrl.reply(id, messages.OnlineSucceed)
	} else {
		ctrl.reply(id, []byte(err.Error()))
	}
	ctrl.log(logger.Debug, "trust node")
}

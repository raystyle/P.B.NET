package node

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/guid"
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
	ctrl := roleCtrl{
		ctx:        server.ctx,
		conn:       conn,
		slots:      make([]*protocol.Slot, protocol.SlotSize),
		replyTimer: time.NewTimer(time.Second),
		rand:       random.New(server.ctx.global.Now().Unix()),
		stopSignal: make(chan struct{}),
	}
	server.addCtrl(&ctrl)
	ctrl.log(logger.Debug, "controller connected")
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("serve controller panic:", r)
			ctrl.log(logger.Exploit, err)
		}
		ctrl.Close()
		server.delCtrl("", &ctrl)
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
	case protocol.CtrlSyncSendToken:
		ctrl.handleSyncSendToken(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncSend:
		ctrl.handleSyncSend(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncRecvToken:
		ctrl.handleSyncReceiveToken(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncRecv:
		ctrl.handleSyncReceive(msg[cmd:id], msg[id:])
	case protocol.CtrlBroadcastToken:
		ctrl.handleBroadcastToken(msg[cmd:id], msg[id:])
	case protocol.CtrlBroadcast:
		ctrl.handleBroadcast(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncQuery:
		ctrl.handleSyncQuery(msg[cmd:id], msg[id:])
	// ---------------------------internal--------------------------------
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
	case protocol.TestCommand:
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

// ----------------------------------sync----------------------------------------

func (ctrl *roleCtrl) handleBroadcastToken(id, message []byte) {
	// role + message guid
	if len(message) != 1+guid.Size {
		// fake reply and close
		ctrl.log(logger.Exploit, "invalid broadcast token size")
		ctrl.reply(id, protocol.BroadcastHandled)
		ctrl.Close()
		return
	}
	role := protocol.Role(message[0])
	if role != protocol.Ctrl {
		ctrl.log(logger.Exploit, "handle invalid broadcast token role")
		ctrl.reply(id, protocol.BroadcastHandled)
		ctrl.Close()
		return
	}
	if ctrl.ctx.syncer.checkBroadcastToken(role, message[1:]) {
		ctrl.reply(id, protocol.BroadcastUnhandled)
	} else {
		ctrl.reply(id, protocol.BroadcastHandled)
	}
}

func (ctrl *roleCtrl) handleSyncSendToken(id, message []byte) {
	// role + message guid
	if len(message) != 1+guid.Size {
		// fake reply and close
		ctrl.log(logger.Exploit, "invalid sync send token size")
		ctrl.reply(id, protocol.SyncHandled)
		ctrl.Close()
		return
	}
	role := protocol.Role(message[0])
	if role != protocol.Ctrl {
		ctrl.log(logger.Exploit, "handle invalid sync send token role")
		ctrl.reply(id, protocol.SyncHandled)
		ctrl.Close()
		return
	}
	if ctrl.ctx.syncer.checkSyncSendToken(role, message[1:]) {
		ctrl.reply(id, protocol.SyncUnhandled)
	} else {
		ctrl.reply(id, protocol.SyncHandled)
	}
}

func (ctrl *roleCtrl) handleSyncReceiveToken(id, message []byte) {
	// role + message guid
	if len(message) != 1+guid.Size {
		// fake reply and close
		ctrl.log(logger.Exploit, "invalid sync receive token size")
		ctrl.reply(id, protocol.SyncHandled)
		ctrl.Close()
		return
	}
	role := protocol.Role(message[0])
	if role != protocol.Ctrl {
		ctrl.log(logger.Exploit, "handle invalid sync receive token role")
		ctrl.reply(id, protocol.SyncHandled)
		ctrl.Close()
		return
	}
	if ctrl.ctx.syncer.checkSyncReceiveToken(role, message[1:]) {
		ctrl.reply(id, protocol.SyncUnhandled)
	} else {
		ctrl.reply(id, protocol.SyncHandled)
	}
}

func (ctrl *roleCtrl) handleBroadcast(id, message []byte) {
	br := protocol.Broadcast{}
	err := msgpack.Unmarshal(message, &br)
	if err != nil {
		ctrl.logln(logger.Exploit, "invalid broadcast msgpack data:", err)
		ctrl.Close()
		return
	}
	err = br.Validate()
	if err != nil {
		ctrl.logf(logger.Exploit, "invalid broadcast: %s\n%s", err, spew.Sdump(br))
		ctrl.Close()
		return
	}
	if br.SenderRole != protocol.Node && br.SenderRole != protocol.Beacon {
		ctrl.logf(logger.Exploit, "invalid broadcast sender role\n%s", spew.Sdump(br))
		ctrl.Close()
		return
	}
	if br.ReceiverRole != protocol.Ctrl {
		ctrl.logf(logger.Exploit, "invalid broadcast receiver role\n%s", spew.Sdump(br))
		ctrl.Close()
		return
	}
	ctrl.ctx.syncer.addBroadcast(&br)
	ctrl.reply(id, protocol.BroadcastSucceed)
}

func (ctrl *roleCtrl) handleSyncSend(id, message []byte) {
	ss := protocol.SyncSend{}
	err := msgpack.Unmarshal(message, &ss)
	if err != nil {
		ctrl.logln(logger.Exploit, "invalid sync send msgpack data:", err)
		ctrl.Close()
		return
	}
	err = ss.Validate()
	if err != nil {
		ctrl.logf(logger.Exploit, "invalid sync send: %s\n%s", err, spew.Sdump(ss))
		ctrl.Close()
		return
	}
	if ss.SenderRole != protocol.Node && ss.SenderRole != protocol.Beacon {
		ctrl.logf(logger.Exploit, "invalid sync send sender role\n%s", spew.Sdump(ss))
		ctrl.Close()
		return
	}
	if ss.ReceiverRole != protocol.Ctrl {
		ctrl.logf(logger.Exploit, "invalid sync send receiver role\n%s", spew.Sdump(ss))
		ctrl.Close()
		return
	}
	if !bytes.Equal(ss.ReceiverGUID, protocol.CtrlGUID) {
		ctrl.logf(logger.Exploit, "invalid sync send receiver guid\n%s", spew.Sdump(ss))
		ctrl.Close()
		return
	}
	ctrl.ctx.syncer.addSyncSend(&ss)
	ctrl.reply(id, protocol.SyncSucceed)
}

// notice controller, role received this height message
func (ctrl *roleCtrl) handleSyncReceive(id, message []byte) {
	sr := protocol.SyncReceive{}
	err := msgpack.Unmarshal(message, &sr)
	if err != nil {
		ctrl.logln(logger.Exploit, "invalid sync receive msgpack data:", err)
		ctrl.Close()
		return
	}
	err = sr.Validate()
	if err != nil {
		ctrl.logf(logger.Exploit, "invalid sync receive: %s\n%s", err, spew.Sdump(sr))
		ctrl.Close()
		return
	}
	if sr.ReceiverRole != protocol.Node && sr.ReceiverRole != protocol.Beacon {
		ctrl.logf(logger.Exploit, "invalid sync receive receiver role\n%s", spew.Sdump(sr))
		ctrl.Close()
		return
	}
	ctrl.ctx.syncer.addSyncReceive(&sr)
	ctrl.reply(id, protocol.SyncSucceed)
}

func (ctrl *roleCtrl) handleSyncQuery(id, message []byte) {

}

// handle trust

func (ctrl *roleCtrl) handleTrustNode(id []byte) {
	ctrl.reply(id, ctrl.ctx.packOnlineRequest())
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

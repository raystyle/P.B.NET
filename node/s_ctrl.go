package node

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
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

type ctrlConn struct {
	ctx *Node

	conn *conn

	heartbeat bytes.Buffer
	rand      *random.Rand
	inSync    int32

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
}

func (s *server) serveCtrl(tag string, conn *conn) {
	ctrl := ctrlConn{
		ctx:        s.ctx,
		conn:       conn,
		rand:       random.New(s.ctx.global.Now().Unix()),
		stopSignal: make(chan struct{}),
	}
	defer func() {
		if r := recover(); r != nil {
			ctrl.log(logger.Exploit, xpanic.Error(r, "server.serveCtrl"))
		}
		ctrl.Close()
		s.deleteCtrlConn(tag)
		conn.log(logger.Debug, "controller disconnected")
	}()
	s.addCtrlConn(tag, &ctrl)
	conn.log(logger.Debug, "controller connected")
	protocol.HandleConn(conn.conn, ctrl.onFrame)
}

// TODO log
func (ctrl *ctrlConn) log(l logger.Level, log ...interface{}) {
	ctrl.ctx.logger.Print(l, "serve-ctrl", log...)
}

func (ctrl *ctrlConn) onFrame(frame []byte) {
	if !ctrl.conn.onFrame(frame) {
		return
	}
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)

	// if sync

	// check command
	switch frame[0] {

	case protocol.CtrlSync:
		ctrl.handleSyncStart(frame[cmd:id])
	case protocol.CtrlTrustNode:
		ctrl.handleTrustNode(frame[cmd:id])
	case protocol.CtrlSetNodeCert:
		ctrl.handleSetCertificate(frame[cmd:id], frame[id:])
	case protocol.ConnSendHeartbeat:
		ctrl.handleHeartbeat()
	default:
		ctrl.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		ctrl.Close()
	}
}

func (ctrl *ctrlConn) handleHeartbeat() {
	// <security> fake traffic like client
	fakeSize := 64 + ctrl.rand.Int(256)
	// size(4 Bytes) + heartbeat(1 byte) + fake data
	ctrl.heartbeat.Reset()
	ctrl.heartbeat.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
	ctrl.heartbeat.WriteByte(protocol.ConnReplyHeartbeat)
	ctrl.heartbeat.Write(ctrl.rand.Bytes(fakeSize))
	// send heartbeat data
	_ = ctrl.conn.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = ctrl.conn.conn.Write(ctrl.heartbeat.Bytes())
}

func (ctrl *ctrlConn) handleSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		// fake reply
		ctrl.log(logger.Exploit, "invalid ctrl send guid size")
		ctrl.conn.Reply(id, protocol.SendReplyHandled)
		return
	}

	if ctrl.ctx.syncer.checkSyncSendToken(role, data[1:]) {
		ctrl.reply(id, protocol.SendReplyUnhandled)
	} else {
		ctrl.reply(id, protocol.SendReplyHandled)
	}
}

func (ctrl *ctrlConn) handleSend(id, data []byte) {

	if ctrl.ctx.syncer.checkSyncSendToken(role, data[1:]) {
		ctrl.reply(id, protocol.SendReplyUnhandled)
	} else {
		ctrl.reply(id, protocol.SendReplyHandled)
	}
}

// Send is used to send message to connected controller
func (ctrl *ctrlConn) Send(guid, message []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Ctrl,
		GUID: protocol.CtrlGUID,
	}
	var reply []byte
	reply, sr.Err = ctrl.conn.Send(protocol.NodeSendGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendReplyUnhandled) {
		sr.Err = protocol.GetSendReplyError(reply)
		return
	}
	reply, sr.Err = ctrl.conn.Send(protocol.NodeSend, message)
	if sr.Err != nil {
		return
	}
	if bytes.Equal(reply, protocol.SendReplySucceed) {
		return
	} else {
		sr.Err = errors.New(string(reply))
		return sr
	}
}

// handle trust node

func (ctrl *ctrlConn) handleTrustNode(id []byte) {
	ctrl.conn.Reply(id, ctrl.ctx.packOnlineRequest())
}

func (ctrl *ctrlConn) handleSetCertificate(id []byte, data []byte) {
	err := ctrl.ctx.global.SetCertificate(data)
	if err == nil {
		ctrl.conn.Reply(id, messages.RegisterSucceed)
		ctrl.log(logger.Debug, "trust node")
	} else {
		ctrl.conn.Reply(id, []byte(err.Error()))
	}
}

func (ctrl *ctrlConn) Status() *xnet.Status {
	return ctrl.conn.Status()
}

func (ctrl *ctrlConn) Close() {
	ctrl.closeOnce.Do(func() {
		atomic.StoreInt32(&ctrl.inClose, 1)
		close(ctrl.stopSignal)
		ctrl.conn.Close()
	})
}

func (ctrl *ctrlConn) isClosing() bool {
	return atomic.LoadInt32(&ctrl.inClose) != 0
}

// if need async handle message must copy msg first
func (ctrl *ctrlConn) handleMessage2(msg []byte) {
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if ctrl.isClosing() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(msg) < id {
		ctrl.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		ctrl.Close()
		return
	}
	switch msg[0] {
	case protocol.CtrlSendToNodeGUID:
		ctrl.handleSyncSendToken(msg[cmd:id], msg[id:])
	case protocol.CtrlSendToNode:
		ctrl.handleSyncSend(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncReceiveToken:
		ctrl.handleSyncReceiveToken(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncReceive:
		ctrl.handleSyncReceive(msg[cmd:id], msg[id:])
	case protocol.CtrlBroadcastGUID:
		ctrl.handleBroadcastToken(msg[cmd:id], msg[id:])
	case protocol.CtrlBroadcast:
		ctrl.handleBroadcast(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncQueryBeacon:
		ctrl.handleSyncQueryBeacon(msg[cmd:id], msg[id:])
	case protocol.CtrlSyncQueryNode:
		ctrl.handleSyncQueryNode(msg[cmd:id], msg[id:])
	// ---------------------------internal--------------------------------
	case protocol.CtrlReply:
		ctrl.handleReply(msg[cmd:])
	case protocol.CtrlHeartbeat:
		ctrl.handleHeartbeat()
	case protocol.CtrlSync:
		ctrl.handleSyncStart(msg[cmd:id])
	case protocol.CtrlTrustNode:
		ctrl.handleTrustNode(msg[cmd:id])
	case protocol.CtrlSetNodeCert:
		ctrl.handleSetCertificate(msg[cmd:id], msg[id:])
	case protocol.ErrCMDRecvNullMsg:
		ctrl.log(logger.Exploit, protocol.ErrRecvNullMsg)
		ctrl.Close()
	case protocol.ErrCMDTooBigMsg:
		ctrl.log(logger.Exploit, protocol.ErrRecvTooBigMsg)
		ctrl.Close()
	case protocol.TestCommand:
		ctrl.reply(msg[cmd:id], msg[id:])
	default:
		ctrl.logln(logger.Exploit, protocol.ErrRecvUnknownCMD, msg)
		ctrl.Close()
	}
}

func (ctrl *ctrlConn) Broadcast(token, message []byte) *protocol.BroadcastResponse {
	br := protocol.BroadcastResponse{}
	br.Role = protocol.Ctrl
	br.GUID = protocol.CtrlGUID
	reply, err := ctrl.conn.Send(protocol.NodeBroadcastToken, token)
	if err != nil {
		br.Err = err
		return &br
	}
	if !bytes.Equal(reply, protocol.BroadcastReplyUnhandled) {
		br.Err = protocol.ErrBroadcastHandled
		return &br
	}
	// broadcast
	reply, err = ctrl.conn.Send(protocol.NodeBroadcast, message)
	if err != nil {
		br.Err = err
		return &br
	}
	if bytes.Equal(reply, protocol.BroadcastReplySucceed) {
		return &br
	} else {
		br.Err = errors.New(string(reply))
		return &br
	}
}

// SyncReceive is used to notice node clean the message
func (ctrl *ctrlConn) SyncReceive(token, message []byte) *protocol.SyncResponse {
	sr := &protocol.SyncResponse{}
	sr.Role = protocol.Ctrl
	sr.GUID = protocol.CtrlGUID
	resp, err := ctrl.conn.Send(protocol.NodeSyncReceiveToken, token)
	if err != nil {
		sr.Err = err
		return sr
	}
	if !bytes.Equal(resp, protocol.SendReplyUnhandled) {
		sr.Err = protocol.ErrSyncHandled
		return sr
	}
	resp, err = ctrl.conn.Send(protocol.NodeSyncReceive, message)
	if err != nil {
		sr.Err = err
		return sr
	}
	if bytes.Equal(resp, protocol.SendReplySucceed) {
		return sr
	} else {
		sr.Err = errors.New(string(resp))
		return sr
	}
}

func (ctrl *ctrlConn) handleSyncStart(id []byte) {
	if ctrl.ctx.syncer.SetCtrlConn(ctrl) {
		ctrl.reply(id, []byte{protocol.NodeSync})
		ctrl.log(logger.Debug, "synchronizing")
	} else {
		ctrl.Close()
	}
}

func (ctrl *ctrlConn) handleBroadcastToken(id, message []byte) {
	// role + message guid
	if len(message) != 1+guid.Size {
		// fake reply and close
		ctrl.log(logger.Exploit, "invalid broadcast token size")
		ctrl.reply(id, protocol.BroadcastReplyHandled)
		ctrl.Close()
		return
	}
	role := protocol.Role(message[0])
	if role != protocol.Ctrl {
		ctrl.log(logger.Exploit, "handle invalid broadcast token role")
		ctrl.reply(id, protocol.BroadcastReplyHandled)
		ctrl.Close()
		return
	}
	if ctrl.ctx.syncer.checkBroadcastToken(role, message[1:]) {
		ctrl.reply(id, protocol.BroadcastReplyUnhandled)
	} else {
		ctrl.reply(id, protocol.BroadcastReplyHandled)
	}
}

func (ctrl *ctrlConn) handleSyncReceiveToken(id, message []byte) {
	// role + message guid
	if len(message) != 1+guid.Size {
		// fake reply and close
		ctrl.log(logger.Exploit, "invalid sync receive token size")
		ctrl.reply(id, protocol.SendReplyHandled)
		ctrl.Close()
		return
	}
	role := protocol.Role(message[0])
	if role != protocol.Ctrl {
		ctrl.log(logger.Exploit, "handle invalid sync receive token role")
		ctrl.reply(id, protocol.SendReplyHandled)
		ctrl.Close()
		return
	}
	if ctrl.ctx.syncer.checkSyncReceiveToken(role, message[1:]) {
		ctrl.reply(id, protocol.SendReplyUnhandled)
	} else {
		ctrl.reply(id, protocol.SendReplyHandled)
	}
}

func (ctrl *ctrlConn) handleBroadcast(id, message []byte) {
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
	if br.SenderRole != protocol.Ctrl {
		ctrl.logf(logger.Exploit, "invalid broadcast sender role\n%s", spew.Sdump(br))
		ctrl.Close()
		return
	}
	if !bytes.Equal(br.SenderGUID, protocol.CtrlGUID) {
		ctrl.logf(logger.Exploit, "invalid broadcast sender guid\n%s", spew.Sdump(br))
		ctrl.Close()
		return
	}
	ctrl.ctx.syncer.addBroadcast(&br)
	ctrl.reply(id, protocol.BroadcastReplySucceed)
}

func (ctrl *ctrlConn) handleSyncSend(id, message []byte) {
	ss := protocol.Send{}
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
	if ss.SenderRole != protocol.Ctrl {
		ctrl.logf(logger.Exploit, "invalid sync send sender role\n%s", spew.Sdump(ss))
		ctrl.Close()
		return
	}
	if !bytes.Equal(ss.SenderGUID, protocol.CtrlGUID) {
		ctrl.logf(logger.Exploit, "invalid sync send sender guid\n%s", spew.Sdump(ss))
		ctrl.Close()
		return
	}
	if ss.ReceiverRole != protocol.Node && ss.ReceiverRole != protocol.Beacon {
		ctrl.logf(logger.Exploit, "invalid sync send receiver role\n%s", spew.Sdump(ss))
		ctrl.Close()
		return
	}
	ctrl.ctx.syncer.addSyncSend(&ss)
	ctrl.reply(id, protocol.SendReplySucceed)
}

// notice node to delete message
// TODO think more
func (ctrl *ctrlConn) handleSyncReceive(id, message []byte) {
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
	if sr.Role != protocol.Node && sr.Role != protocol.Beacon {
		ctrl.logf(logger.Exploit, "invalid sync receive receiver role\n%s", spew.Sdump(sr))
		ctrl.Close()
		return
	}
	ctrl.ctx.syncer.addSyncReceive(&sr)
	ctrl.reply(id, protocol.SendReplySucceed)
}

func (ctrl *ctrlConn) handleSyncQueryBeacon(id, message []byte) {
	sr := protocol.SyncQuery{}
	err := msgpack.Unmarshal(message, &sr)
	if err != nil {
		ctrl.logln(logger.Exploit, "invalid sync query msgpack data:", err)
		ctrl.Close()
		return
	}
	err = sr.Validate()
	if err != nil {
		ctrl.logf(logger.Exploit, "invalid sync query beacon: %s\n%s", err, spew.Sdump(sr))
		ctrl.Close()
		return
	}
	// TODO reply
	ctrl.reply(id, protocol.SendReplySucceed)
}

func (ctrl *ctrlConn) handleSyncQueryNode(id, message []byte) {
	sr := protocol.SyncQuery{}
	err := msgpack.Unmarshal(message, &sr)
	if err != nil {
		ctrl.logln(logger.Exploit, "invalid sync query msgpack data:", err)
		ctrl.Close()
		return
	}
	err = sr.Validate()
	if err != nil {
		ctrl.logf(logger.Exploit, "invalid sync query node: %s\n%s", err, spew.Sdump(sr))
		ctrl.Close()
		return
	}
	// TODO reply
	ctrl.reply(id, protocol.SendReplySucceed)
}

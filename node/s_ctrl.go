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

	tag  string
	conn *conn

	heartbeat bytes.Buffer
	rand      *random.Rand
	inSync    int32

	sendPool        sync.Pool
	acknowledgePool sync.Pool
	broadcastPool   sync.Pool
	answerPool      sync.Pool

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
}

func (s *server) serveCtrl(tag string, conn *conn) {
	ctrl := ctrlConn{
		ctx:        s.ctx,
		tag:        tag,
		conn:       conn,
		rand:       random.New(s.ctx.global.Now().Unix()),
		stopSignal: make(chan struct{}),
	}

	ctrl.sendPool.New = func() interface{} {
		return new(protocol.Send)
	}
	ctrl.acknowledgePool.New = func() interface{} {
		return new(protocol.Acknowledge)
	}
	ctrl.broadcastPool.New = func() interface{} {
		return new(protocol.Broadcast)
	}
	ctrl.answerPool.New = func() interface{} {
		return new(protocol.Answer)
	}

	defer func() {
		if r := recover(); r != nil {
			ctrl.log(logger.Exploit, xpanic.Error(r, "server.serveCtrl"))
		}
		ctrl.Close()
		if ctrl.isSync() {
			s.ctx.forwarder.LogoffCtrl(tag)
		}
		s.deleteCtrlConn(tag)
		conn.log(logger.Debug, "controller disconnected")
	}()
	s.addCtrlConn(tag, &ctrl)
	conn.log(logger.Debug, "controller connected")
	protocol.HandleConn(conn.conn, ctrl.onFrame)
}

func (ctrl *ctrlConn) isSync() bool {
	return atomic.LoadInt32(&ctrl.inSync) != 0
}

func (ctrl *ctrlConn) isClosing() bool {
	return atomic.LoadInt32(&ctrl.inClose) != 0
}

// TODO log
func (ctrl *ctrlConn) log(l logger.Level, log ...interface{}) {
	ctrl.ctx.logger.Print(l, "serve-ctrl", log...)
}

func (ctrl *ctrlConn) logf(l logger.Level, format string, log ...interface{}) {
	ctrl.ctx.logger.Printf(l, "serve-ctrl", format, log...)
}

func (ctrl *ctrlConn) onFrame(frame []byte) {
	if ctrl.isClosing() {
		return
	}
	if ctrl.conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnSendHeartbeat {
		ctrl.handleHeartbeat()
		return
	}
	id := frame[protocol.MsgCMDSize : protocol.MsgCMDSize+protocol.MsgIDSize]
	data := frame[protocol.MsgCMDSize+protocol.MsgIDSize:]
	if ctrl.isSync() {
		if ctrl.onFrameAfterSync(frame[0], id, data) {
			return
		}
	} else {
		if ctrl.onFrameBeforeSync(frame[0], id, data) {
			return
		}
	}
	ctrl.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
	ctrl.Close()
}

func (ctrl *ctrlConn) onFrameBeforeSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSync:
		ctrl.handleSyncStart(id)
	case protocol.CtrlTrustNode:
		ctrl.handleTrustNode(id)
	case protocol.CtrlSetNodeCert:
		ctrl.handleSetCertificate(id, data)
	default:
		return false
	}
	return true
}

func (ctrl *ctrlConn) onFrameAfterSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSendToNodeGUID:
		ctrl.handleSendGUID(id, data)
	case protocol.CtrlSendToNode:
		ctrl.handleSend(id, data, protocol.Node)
	case protocol.CtrlAckToNodeGUID:
		ctrl.handleAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		ctrl.handleAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		ctrl.handleSendGUID(id, data)
	case protocol.CtrlSendToBeacon:
		ctrl.handleSend(id, data, protocol.Beacon)
	case protocol.CtrlAckToBeaconGUID:
		ctrl.handleAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		ctrl.handleAckToBeacon(id, data)
	case protocol.CtrlBroadcastGUID:
		ctrl.handleBroadcastGUID(id, data)
	case protocol.CtrlBroadcast:
		ctrl.handleBroadcast(id, data)
	case protocol.CtrlAnswerGUID:
		ctrl.handleAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		ctrl.handleAnswer(id, data)
	default:
		return false
	}
	return true
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

func (ctrl *ctrlConn) handleSyncStart(id []byte) {
	if ctrl.isSync() {
		return
	}
	err := ctrl.ctx.forwarder.RegisterCtrl(ctrl.tag, ctrl)
	if err != nil {
		ctrl.conn.Reply(id, []byte(err.Error()))
		ctrl.Close()
	} else {
		atomic.StoreInt32(&ctrl.inSync, 1)
		ctrl.conn.Reply(id, []byte{protocol.NodeSync})
		ctrl.log(logger.Debug, "synchronizing")
	}
}

func (ctrl *ctrlConn) handleSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		ctrl.log(logger.Exploit, "invalid send guid size")
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.Close()
		return
	}
	if expired, _ := ctrl.ctx.syncer.CheckGUIDTimestamp(data); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckCtrlSendGUID(data, false, 0) {
		ctrl.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleBroadcastGUID(id, data []byte) {
	if len(data) != guid.Size {
		ctrl.log(logger.Exploit, "invalid broadcast guid size")
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.Close()
		return
	}
	if expired, _ := ctrl.ctx.syncer.CheckGUIDTimestamp(data); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckBroadcastGUID(data, false, 0) {
		ctrl.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleAckToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		ctrl.log(logger.Exploit, "invalid acknowledge to node guid size")
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.Close()
		return
	}
	if expired, _ := ctrl.ctx.syncer.CheckGUIDTimestamp(data); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckCtrlAckToNodeGUID(data, false, 0) {
		ctrl.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleAckToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		ctrl.log(logger.Exploit, "invalid acknowledge to beacon guid size")
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.Close()
		return
	}
	if expired, _ := ctrl.ctx.syncer.CheckGUIDTimestamp(data); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckCtrlAckToBeaconGUID(data, false, 0) {
		ctrl.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleAnswerGUID(id, data []byte) {
	if len(data) != guid.Size {
		ctrl.log(logger.Exploit, "invalid answer guid size")
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.Close()
		return
	}
	if expired, _ := ctrl.ctx.syncer.CheckGUIDTimestamp(data); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckAnswerGUID(data, false, 0) {
		ctrl.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleSend(id, data []byte, role protocol.Role) {
	s := ctrl.sendPool.Get().(*protocol.Send)
	defer ctrl.sendPool.Put(s)

	err := msgpack.Unmarshal(data, s)
	if err != nil {
		const format = "invalid send msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid send: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(s))
		ctrl.Close()
		return
	}
	if expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(s.GUID); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckCtrlSendGUID(s.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		switch role {
		case protocol.Node:
			if bytes.Equal(s.RoleGUID, ctrl.ctx.global.GUID()) {
				ns := ctrl.ctx.worker.GetSendFromPool()
				*ns = *s // must copy, because sync.Pool
				ctrl.ctx.worker.AddSend(ns)
			} else { // repeat

			}
		case protocol.Beacon:

		}
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleBroadcast(id, data []byte) {
	b := ctrl.broadcastPool.Get().(*protocol.Broadcast)
	defer ctrl.broadcastPool.Put(b)

	err := msgpack.Unmarshal(data, b)
	if err != nil {
		const format = "invalid ctrl broadcast msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.Close()
		return
	}
	err = b.Validate()
	if err != nil {
		const format = "invalid ctrl broadcast: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(b))
		ctrl.Close()
		return
	}
	if expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(b.GUID); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckBroadcastGUID(b.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		nb := ctrl.ctx.worker.GetBroadcastFromPool()
		*nb = *b // must copy, because sync.Pool
		ctrl.ctx.worker.AddBroadcast(nb)
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleAckToNode(id, data []byte) {
	a := ctrl.acknowledgePool.Get().(*protocol.Acknowledge)
	defer ctrl.acknowledgePool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid acknowledge to node msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid acknowledge to node: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(a))
		ctrl.Close()
		return
	}
	if expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(a.GUID); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckCtrlAckToNodeGUID(a.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleAckToBeacon(id, data []byte) {
	a := ctrl.acknowledgePool.Get().(*protocol.Acknowledge)
	defer ctrl.acknowledgePool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid acknowledge to beacon msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid acknowledge to beacon: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(a))
		ctrl.Close()
		return
	}
	if expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(a.GUID); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckCtrlAckToBeaconGUID(a.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

// TODO may be copy send data
func (ctrl *ctrlConn) handleAnswer(id, data []byte) {
	a := ctrl.answerPool.Get().(*protocol.Answer)
	defer ctrl.answerPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid answer msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid answer: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(a))
		ctrl.Close()
		return
	}
	if expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(a.GUID); expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
	} else if ctrl.ctx.syncer.CheckAnswerGUID(a.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

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
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = ctrl.conn.Send(protocol.NodeSend, message)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

// Acknowledge is used to acknowledge to controller
func (ctrl *ctrlConn) Acknowledge(guid, message []byte) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Ctrl,
		GUID: protocol.CtrlGUID,
	}
	var reply []byte
	reply, ar.Err = ctrl.conn.Send(protocol.NodeAckGUID, guid)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		ar.Err = protocol.GetReplyError(reply)
		return
	}
	reply, ar.Err = ctrl.conn.Send(protocol.NodeAck, message)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		ar.Err = errors.New(string(reply))
	}
	return
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

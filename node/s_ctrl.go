package node

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
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
	answerPool      sync.Pool

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
}

func (s *server) serveCtrl(tag string, conn *conn) {
	ctrlConn := ctrlConn{
		ctx:        s.ctx,
		tag:        tag,
		conn:       conn,
		rand:       random.New(),
		stopSignal: make(chan struct{}),
	}

	ctrlConn.sendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	ctrlConn.acknowledgePool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	ctrlConn.answerPool.New = func() interface{} {
		return &protocol.Answer{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Message:    make([]byte, aes.BlockSize),
			Hash:       make([]byte, sha256.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}

	defer func() {
		if r := recover(); r != nil {
			ctrlConn.log(logger.Exploit, xpanic.Error(r, "server.serveCtrl"))
		}
		ctrlConn.Close()
		if ctrlConn.isSync() {
			s.ctx.forwarder.LogoffCtrl(tag)
		}
		s.deleteCtrlConn(tag)
		ctrlConn.log(logger.Debug, "controller disconnected")
	}()
	s.addCtrlConn(tag, &ctrlConn)
	_ = conn.conn.SetDeadline(s.ctx.global.Now().Add(s.timeout))
	ctrlConn.log(logger.Debug, "controller connected")
	protocol.HandleConn(conn.conn, ctrlConn.onFrame)
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
		ctrl.handleSendToNode(id, data)
	case protocol.CtrlAckToNodeGUID:
		ctrl.handleAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		ctrl.handleAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		ctrl.handleSendGUID(id, data)
	case protocol.CtrlSendToBeacon:
		ctrl.handleSendToBeacon(id, data)
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

func (ctrl *ctrlConn) handleSendToNode(id, data []byte) {
	s := ctrl.ctx.worker.GetSendFromPool()
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		const format = "invalid send to node msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.ctx.worker.PutSendToPool(s)
		ctrl.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid send to node: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(s))
		ctrl.ctx.worker.PutSendToPool(s)
		ctrl.Close()
		return
	}
	expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
		ctrl.ctx.worker.PutSendToPool(s)
		return
	}
	if ctrl.ctx.syncer.CheckCtrlSendGUID(s.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		if bytes.Equal(s.RoleGUID, ctrl.ctx.global.GUID()) {
			ctrl.ctx.worker.AddSend(s)
		} else {
			// repeat
			ctrl.ctx.worker.PutSendToPool(s)
		}
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.ctx.worker.PutSendToPool(s)
	}
}

func (ctrl *ctrlConn) handleSendToBeacon(id, data []byte) {
	s := ctrl.sendPool.Get().(*protocol.Send)
	defer ctrl.sendPool.Put(s)
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		const format = "invalid send to beacon msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid send to beacon: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(s))
		ctrl.Close()
		return
	}
	expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if ctrl.ctx.syncer.CheckCtrlSendGUID(s.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (ctrl *ctrlConn) handleBroadcast(id, data []byte) {
	b := ctrl.ctx.worker.GetBroadcastFromPool()
	err := msgpack.Unmarshal(data, b)
	if err != nil {
		const format = "invalid ctrl broadcast msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.ctx.worker.PutBroadcastToPool(b)
		ctrl.Close()
		return
	}
	err = b.Validate()
	if err != nil {
		const format = "invalid ctrl broadcast: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(b))
		ctrl.ctx.worker.PutBroadcastToPool(b)
		ctrl.Close()
		return
	}
	expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(b.GUID)
	if expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
		ctrl.ctx.worker.PutBroadcastToPool(b)
		return
	}
	if ctrl.ctx.syncer.CheckBroadcastGUID(b.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		ctrl.ctx.worker.AddBroadcast(b)
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.ctx.worker.PutBroadcastToPool(b)
	}
}

// TODO warning repeat
func (ctrl *ctrlConn) handleAckToNode(id, data []byte) {
	a := ctrl.ctx.worker.GetAcknowledgeFromPool()

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid acknowledge to node msgpack data: %s"
		ctrl.logf(logger.Exploit, format, err)
		ctrl.ctx.worker.PutAcknowledgeToPool(a)
		ctrl.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid acknowledge to node: %s\n%s"
		ctrl.logf(logger.Exploit, format, err, spew.Sdump(a))
		ctrl.ctx.worker.PutAcknowledgeToPool(a)
		ctrl.Close()
		return
	}
	expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
		ctrl.ctx.worker.PutAcknowledgeToPool(a)
		return
	}
	if ctrl.ctx.syncer.CheckCtrlAckToNodeGUID(a.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		if bytes.Equal(a.RoleGUID, ctrl.ctx.global.GUID()) {
			ctrl.ctx.worker.AddAcknowledge(a)

		} else {
			// repeat
			ctrl.ctx.worker.PutAcknowledgeToPool(a)
		}
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
		ctrl.ctx.worker.PutAcknowledgeToPool(a)
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
	expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if ctrl.ctx.syncer.CheckCtrlAckToBeaconGUID(a.GUID, true, timestamp) {
		ctrl.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		ctrl.conn.Reply(id, protocol.ReplyHandled)
	}
}

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
	expired, timestamp := ctrl.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		ctrl.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if ctrl.ctx.syncer.CheckAnswerGUID(a.GUID, true, timestamp) {
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
		ctrl.conn.Reply(id, []byte{messages.RegisterResultAccept})
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

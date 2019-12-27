package node

import (
	"bytes"
	"crypto/sha256"
	"fmt"
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
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
	"project/internal/xpanic"
)

type nodeConn struct {
	ctx *Node

	tag  string
	guid []byte // node guid
	conn *conn

	heartbeat bytes.Buffer
	rand      *random.Rand
	inSync    int32

	sendPool   sync.Pool
	ackPool    sync.Pool
	answerPool sync.Pool
	queryPool  sync.Pool

	closeOnce sync.Once
}

func (s *server) serveNode(tag string, nodeGUID []byte, conn *conn) {
	nc := nodeConn{
		ctx:  s.ctx,
		tag:  tag,
		guid: nodeGUID,
		conn: conn,
		rand: random.New(),
	}
	defer func() {
		if r := recover(); r != nil {
			nc.log(logger.Exploit, xpanic.Error(r, "server.serveNode"))
		}
		nc.Close()
		if nc.isSync() {
			s.ctx.forwarder.LogoffNode(tag)
		}
		s.deleteNodeConn(tag)
		nc.logf(logger.Debug, "node %X disconnected", nodeGUID)
	}()
	s.addNodeConn(tag, &nc)
	_ = conn.SetDeadline(s.ctx.global.Now().Add(s.timeout))
	nc.logf(logger.Debug, "node %X connected", nodeGUID)
	protocol.HandleConn(conn, nc.onFrame)
}

// TODO add guid
func (node *nodeConn) log(l logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintln(b, log...)
	_, _ = fmt.Fprint(b, "\n", node.conn)
	node.ctx.logger.Print(l, "serve-node", b)
}

func (node *nodeConn) logf(l logger.Level, format string, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, format, log...)
	_, _ = fmt.Fprint(b, "\n\n", node.conn)
	node.ctx.logger.Print(l, "serve-node", b)
}

func (node *nodeConn) isSync() bool {
	return atomic.LoadInt32(&node.inSync) != 0
}

func (node *nodeConn) onFrame(frame []byte) {
	if node.conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnSendHeartbeat {
		node.handleHeartbeat()
		return
	}
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	if node.isSync() {
		if node.onFrameAfterSync(frame[0], id, data) {
			return
		}
	} else {
		if node.onFrameBeforeSync(frame[0], id, data) {
			return
		}
	}
	const format = "unknown command: %d\nframe:\n%s"
	node.logf(logger.Exploit, format, frame[0], spew.Sdump(frame))
	node.Close()
}

func (node *nodeConn) onFrameBeforeSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.NodeSync:
		node.handleSyncStart(id)
	default:
		return false
	}
	return true
}

func (node *nodeConn) onFrameAfterSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSendToNodeGUID:
		node.handleSendToNodeGUID(id, data)
	case protocol.CtrlSendToNode:
		node.handleSendToNode(id, data)
	case protocol.CtrlAckToNodeGUID:
		node.handleAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		node.handleAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		node.handleSendToBeaconGUID(id, data)
	case protocol.CtrlSendToBeacon:
		node.handleSendToBeacon(id, data)
	case protocol.CtrlAckToBeaconGUID:
		node.handleAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		node.handleAckToBeacon(id, data)
	case protocol.CtrlBroadcastGUID:
		node.handleBroadcastGUID(id, data)
	case protocol.CtrlBroadcast:
		node.handleBroadcast(id, data)
	case protocol.CtrlAnswerGUID:
		node.handleAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		node.handleAnswer(id, data)
	case protocol.NodeSendGUID:
		node.handleNodeSendGUID(id, data)
	case protocol.NodeSend:
		node.handleNodeSend(id, data)
	case protocol.NodeAckGUID:
		node.handleNodeAckGUID(id, data)
	case protocol.NodeAck:
		node.handleNodeAck(id, data)
	case protocol.BeaconSendGUID:
		node.handleBeaconSendGUID(id, data)
	case protocol.BeaconSend:
		node.handleBeaconSend(id, data)
	case protocol.BeaconAckGUID:
		node.handleBeaconAckGUID(id, data)
	case protocol.BeaconAck:
		node.handleBeaconAck(id, data)
	case protocol.BeaconQueryGUID:
		node.handleBeaconQueryGUID(id, data)
	case protocol.BeaconQuery:
		node.handleBeaconQuery(id, data)
	default:
		return false
	}
	return true
}

func (node *nodeConn) handleHeartbeat() {
	// <security> fake traffic like client
	fakeSize := 64 + node.rand.Int(256)
	// size(4 Bytes) + heartbeat(1 byte) + fake data
	node.heartbeat.Reset()
	node.heartbeat.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
	node.heartbeat.WriteByte(protocol.ConnReplyHeartbeat)
	node.heartbeat.Write(node.rand.Bytes(fakeSize))
	// send heartbeat data
	_ = node.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = node.conn.Write(node.heartbeat.Bytes())
}

func (node *nodeConn) handleSyncStart(id []byte) {
	if node.isSync() {
		return
	}
	node.sendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	node.ackPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	node.answerPool.New = func() interface{} {
		return &protocol.Answer{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Message:    make([]byte, aes.BlockSize),
			Hash:       make([]byte, sha256.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	node.queryPool.New = func() interface{} {
		return &protocol.Query{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	err := node.ctx.forwarder.RegisterNode(node.tag, node)
	if err != nil {
		node.conn.Reply(id, []byte(err.Error()))
		node.Close()
	} else {
		atomic.StoreInt32(&node.inSync, 1)
		node.conn.Reply(id, []byte{protocol.NodeSync})
		node.log(logger.Debug, "synchronizing")
	}
}

func (node *nodeConn) handleSendToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid send to node guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckSendToNodeGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleSendToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid send to beacon guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckSendToBeaconGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleAckToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid ack to node guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckAckToNodeGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleAckToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid ack to beacon guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckAckToBeaconGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBroadcastGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid broadcast guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckBroadcastGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleAnswerGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid answer guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckAnswerGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleNodeSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid node send guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckNodeSendGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleNodeAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid node ack guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckNodeAckGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBeaconSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid beacon send guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckBeaconSendGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBeaconAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid beacon ack guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckBeaconAckGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBeaconQueryGUID(id, data []byte) {
	if len(data) != guid.Size {
		node.log(logger.Exploit, "invalid beacon query guid size")
		node.conn.Reply(id, protocol.ReplyHandled)
		node.Close()
		return
	}
	if expired, _ := node.ctx.syncer.CheckGUIDTimestamp(data); expired {
		node.conn.Reply(id, protocol.ReplyExpired)
	} else if node.ctx.syncer.CheckQueryGUID(data, false, 0) {
		node.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleSendToNode(id, data []byte) {
	s := node.ctx.worker.GetSendFromPool()
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		const format = "invalid send to node msgpack data: %s"
		node.logf(logger.Exploit, format, err)
		node.ctx.worker.PutSendToPool(s)
		node.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid send to node: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(s))
		node.ctx.worker.PutSendToPool(s)
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		node.ctx.worker.PutSendToPool(s)
		return
	}
	if node.ctx.syncer.CheckSendToNodeGUID(s.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
		if bytes.Equal(s.RoleGUID, node.ctx.global.GUID()) {
			node.ctx.worker.AddSend(s)
		} else {
			// repeat

			node.ctx.worker.PutSendToPool(s)
		}
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
		node.ctx.worker.PutSendToPool(s)
	}
}

func (node *nodeConn) handleAckToNode(id, data []byte) {
	a := node.ctx.worker.GetAcknowledgeFromPool()

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid ack to node msgpack data: %s"
		node.logf(logger.Exploit, format, err)
		node.ctx.worker.PutAcknowledgeToPool(a)
		node.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid ack to node: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(a))
		node.ctx.worker.PutAcknowledgeToPool(a)
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		node.ctx.worker.PutAcknowledgeToPool(a)
		return
	}
	if node.ctx.syncer.CheckAckToNodeGUID(a.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
		if bytes.Equal(a.RoleGUID, node.ctx.global.GUID()) {
			node.ctx.worker.AddAcknowledge(a)

		} else {
			// repeat
			node.ctx.worker.PutAcknowledgeToPool(a)
		}
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
		node.ctx.worker.PutAcknowledgeToPool(a)
	}
}

func (node *nodeConn) handleSendToBeacon(id, data []byte) {
	s := node.sendPool.Get().(*protocol.Send)
	defer node.sendPool.Put(s)
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		const format = "invalid send to beacon msgpack data: %s"
		node.logf(logger.Exploit, format, err)
		node.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid send to beacon: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(s))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckSendToBeaconGUID(s.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleAckToBeacon(id, data []byte) {
	a := node.ackPool.Get().(*protocol.Acknowledge)
	defer node.ackPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid ack to beacon msgpack data: %s"
		node.logf(logger.Exploit, format, err)
		node.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid ack to beacon: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(a))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckAckToBeaconGUID(a.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBroadcast(id, data []byte) {
	b := node.ctx.worker.GetBroadcastFromPool()
	err := msgpack.Unmarshal(data, b)
	if err != nil {
		const format = "invalid broadcast msgpack data: %s"
		node.logf(logger.Exploit, format, err)
		node.ctx.worker.PutBroadcastToPool(b)
		node.Close()
		return
	}
	err = b.Validate()
	if err != nil {
		const format = "invalid broadcast: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(b))
		node.ctx.worker.PutBroadcastToPool(b)
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(b.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		node.ctx.worker.PutBroadcastToPool(b)
		return
	}
	if node.ctx.syncer.CheckBroadcastGUID(b.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
		node.ctx.worker.AddBroadcast(b)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
		node.ctx.worker.PutBroadcastToPool(b)
	}
}

func (node *nodeConn) handleAnswer(id, data []byte) {
	a := node.answerPool.Get().(*protocol.Answer)
	defer node.answerPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid answer msgpack data: %s"
		node.logf(logger.Exploit, format, err)
		node.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid answer: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(a))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckAnswerGUID(a.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleNodeSend(id, data []byte) {
	s := node.sendPool.Get().(*protocol.Send)
	defer node.sendPool.Put(s)

	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		node.log(logger.Exploit, "invalid node send msgpack data:", err)
		node.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid node send: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(s))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckNodeSendGUID(s.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleNodeAck(id, data []byte) {
	a := node.ackPool.Get().(*protocol.Acknowledge)
	defer node.ackPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		node.log(logger.Exploit, "invalid node ack msgpack data:", err)
		node.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid node ack: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(a))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckNodeAckGUID(a.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBeaconSend(id, data []byte) {
	s := node.sendPool.Get().(*protocol.Send)
	defer node.sendPool.Put(s)

	err := msgpack.Unmarshal(data, s)
	if err != nil {
		node.log(logger.Exploit, "invalid beacon send msgpack data:", err)
		node.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid beacon send: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(s))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckBeaconSendGUID(s.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBeaconAck(id, data []byte) {
	a := node.ackPool.Get().(*protocol.Acknowledge)
	defer node.ackPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		node.log(logger.Exploit, "invalid beacon ack msgpack data:", err)
		node.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid beacon ack: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(a))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckBeaconAckGUID(a.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (node *nodeConn) handleBeaconQuery(id, data []byte) {
	q := node.queryPool.Get().(*protocol.Query)
	defer node.queryPool.Put(q)

	err := msgpack.Unmarshal(data, q)
	if err != nil {
		node.log(logger.Exploit, "invalid query msgpack data:", err)
		node.Close()
		return
	}
	err = q.Validate()
	if err != nil {
		const format = "invalid query: %s\n%s"
		node.logf(logger.Exploit, format, err, spew.Sdump(q))
		node.Close()
		return
	}
	expired, timestamp := node.ctx.syncer.CheckGUIDTimestamp(q.GUID)
	if expired {
		node.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if node.ctx.syncer.CheckQueryGUID(q.GUID, true, timestamp) {
		node.conn.Reply(id, protocol.ReplySucceed)
	} else {
		node.conn.Reply(id, protocol.ReplyHandled)
	}
}

// Send is used to send message to connected controller
func (node *nodeConn) Send(guid, message []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: node.guid,
	}
	var reply []byte
	reply, sr.Err = node.conn.Send(protocol.NodeSendGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = node.conn.Send(protocol.NodeSend, message)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

// Acknowledge is used to acknowledge to controller
func (node *nodeConn) Acknowledge(guid, message []byte) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: node.guid,
	}
	var reply []byte
	reply, ar.Err = node.conn.Send(protocol.NodeAckGUID, guid)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		ar.Err = protocol.GetReplyError(reply)
		return
	}
	reply, ar.Err = node.conn.Send(protocol.NodeAck, message)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		ar.Err = errors.New(string(reply))
	}
	return
}

// Status is used to get connection status
func (node *nodeConn) Status() *xnet.Status {
	return node.conn.Status()
}

// Close is used to stop serve node
func (node *nodeConn) Close() {
	node.closeOnce.Do(func() {
		_ = node.conn.Close()
	})
}

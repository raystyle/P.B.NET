package node

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

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
	guid []byte
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

func (s *server) serveNode(tag string, nodeGUID []byte, conn *conn) {
	nodeConn := nodeConn{
		ctx:        s.ctx,
		tag:        tag,
		guid:       nodeGUID,
		conn:       conn,
		rand:       random.New(),
		stopSignal: make(chan struct{}),
	}

	nodeConn.sendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	nodeConn.acknowledgePool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	nodeConn.answerPool.New = func() interface{} {
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
			nodeConn.log(logger.Exploit, xpanic.Error(r, "server.serveNode"))
		}
		nodeConn.Close()
		if nodeConn.isSync() {
			s.ctx.forwarder.LogoffNode(tag)
		}
		s.deleteNodeConn(tag)
		nodeConn.log(logger.Debug, "node disconnected")
	}()
	s.addNodeConn(tag, &nodeConn)
	_ = conn.conn.SetDeadline(s.ctx.global.Now().Add(s.timeout))
	nodeConn.logf(logger.Debug, "node %X connected", nodeGUID)
	protocol.HandleConn(conn.conn, nodeConn.onFrame)
}

func (node *nodeConn) isSync() bool {
	return atomic.LoadInt32(&node.inSync) != 0
}

func (node *nodeConn) isClosing() bool {
	return atomic.LoadInt32(&node.inClose) != 0
}

// TODO log
func (node *nodeConn) log(l logger.Level, log ...interface{}) {
	node.ctx.logger.Print(l, "serve-node", log...)
}

func (node *nodeConn) logf(l logger.Level, format string, log ...interface{}) {
	node.ctx.logger.Printf(l, "serve-node", format, log...)
}

func (node *nodeConn) onFrame(frame []byte) {
	if node.isClosing() {
		return
	}
	if node.conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnSendHeartbeat {
		node.handleHeartbeat()
		return
	}

	node.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
	node.Close()
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
	_ = node.conn.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = node.conn.conn.Write(node.heartbeat.Bytes())
}

func (node *nodeConn) handleSyncStart(id []byte) {
	if node.isSync() {
		return
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

func (node *nodeConn) Status() *xnet.Status {
	return node.conn.Status()
}

func (node *nodeConn) Close() {
	node.closeOnce.Do(func() {
		atomic.StoreInt32(&node.inClose, 1)
		close(node.stopSignal)
		node.conn.Close()
	})
}

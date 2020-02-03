package node

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
	"project/internal/xnet/xnetutil"
)

const (
	connUsageServeCtrl = iota
	connUsageServeNode
	connUsageServeBeacon
	connUsageClient
)

type conn struct {
	ctx *Node

	*xnet.Conn
	role protocol.Role
	tag  *guid.GUID // connection tag
	guid *guid.GUID // role guid

	slots []*protocol.Slot

	// for log about role guid
	guidLine string
	logSrc   string

	// only serve role
	heartbeat *bytes.Buffer
	rand      *random.Rand

	// user will initialize it in role.Sync()
	SendPool   sync.Pool
	AckPool    sync.Pool
	AnswerPool sync.Pool
	QueryPool  sync.Pool

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
}

func newConn(ctx *Node, xConn *xnet.Conn, tag, guid *guid.GUID, usage int) *conn {
	conn := conn{
		ctx:        ctx,
		Conn:       xConn,
		tag:        tag,
		guid:       guid,
		stopSignal: make(chan struct{}),
	}
	_ = xConn.SetDeadline(time.Time{})
	// initialize message slots
	conn.slots = make([]*protocol.Slot, protocol.SlotSize)
	for i := 0; i < protocol.SlotSize; i++ {
		conn.slots[i] = protocol.NewSlot()
	}
	switch usage {
	case connUsageServeCtrl:
		conn.role = protocol.Ctrl
		conn.logSrc = "serve-ctrl"
	case connUsageServeNode:
		conn.role = protocol.Node
		conn.guidLine = "--------------connected node guid---------------"
		conn.logSrc = "serve-node"
	case connUsageServeBeacon:
		conn.role = protocol.Beacon
		conn.guidLine = "-------------connected beacon guid--------------"
		conn.logSrc = "serve-beacon"
	case connUsageClient:
		conn.role = protocol.Node
		conn.guidLine = "--------------connected node guid---------------"
		conn.logSrc = "client"
	default:
		panic(fmt.Sprintf("invalid conn usage: %d", usage))
	}
	if usage != connUsageServeCtrl {
		conn.guidLine += "\n%s\n"
	}
	// only serve role handle heartbeat
	if usage != connUsageClient {
		conn.heartbeat = bytes.NewBuffer(nil)
		conn.rand = random.New()
	}
	return &conn
}

// [2019-12-26 21:44:17] [info] <client> disconnected
// --------------connected node guid---------------
// F50B876BE94437E2E678C5EB84627230C599B847BED5B00D
// C38C4E155C0DD0305F7A000000005E04B92C000000000000
// ---------------connection status----------------
// local:  tcp 127.0.0.1:2035
// remote: tcp 127.0.0.1:2032
// sent:   5.656 MB, received: 5.379 MB
// mode:   tls,  default network: tcp
// connect time: 2019-12-26 21:44:13
// ------------------------------------------------
func (c *conn) Logf(lv logger.Level, format string, log ...interface{}) {
	output := new(bytes.Buffer)
	_, _ = fmt.Fprintf(output, format+"\n", log...)
	c.logExtra(lv, output)
}

func (c *conn) Log(lv logger.Level, log ...interface{}) {
	output := new(bytes.Buffer)
	_, _ = fmt.Fprintln(output, log...)
	c.logExtra(lv, output)
}

func (c *conn) logExtra(lv logger.Level, buf *bytes.Buffer) {
	if c.role != protocol.Ctrl && *protocol.CtrlGUID != *c.guid {
		_, _ = fmt.Fprintf(buf, c.guidLine, c.guid.Hex())
	}
	const conn = "---------------connection status----------------\n%s\n"
	_, _ = fmt.Fprintf(buf, conn, c)
	const endLine = "------------------------------------------------"
	_, _ = fmt.Fprint(buf, endLine)
	c.ctx.logger.Print(lv, c.logSrc, buf)
}

func (c *conn) isClosed() bool {
	return atomic.LoadInt32(&c.inClose) != 0
}

func (c *conn) onFrame(frame []byte) bool {
	if c.isClosed() {
		return true
	}
	// cmd(1) + msg id(2)
	if len(frame) < protocol.FrameCMDSize+protocol.FrameIDSize {
		c.Log(logger.Exploit, protocol.ErrInvalidFrameSize)
		_ = c.Close()
		return true
	}
	switch frame[0] {
	case protocol.ConnReply:
		c.handleReply(frame[protocol.FrameCMDSize:])
	case protocol.ConnGetAddress:
		id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
		address := xnetutil.EncodeExternalAddress(c.Conn.RemoteAddr().String())
		c.Reply(id, address)
	case protocol.ConnErrRecvNullFrame:
		c.Log(logger.Exploit, protocol.ErrRecvNullFrame)
		_ = c.Close()
	case protocol.ConnErrRecvTooBigFrame:
		c.Log(logger.Exploit, protocol.ErrRecvTooBigFrame)
		_ = c.Close()
	case protocol.TestCommand:
		id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
		data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
		c.Reply(id, data)
	default:
		return false
	}
	return true
}

// msg id(2 bytes) + data
func (c *conn) handleReply(reply []byte) {
	l := len(reply)
	if l < protocol.FrameIDSize {
		c.Log(logger.Exploit, protocol.ErrRecvInvalidFrameIDSize)
		_ = c.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:protocol.FrameIDSize]))
	if id > protocol.MaxFrameID {
		c.Log(logger.Exploit, protocol.ErrRecvInvalidFrameID)
		_ = c.Close()
		return
	}
	// must copy
	r := make([]byte, l-protocol.FrameIDSize)
	copy(r, reply[protocol.FrameIDSize:])
	// <security> maybe incorrect msg id
	select {
	case c.slots[id].Reply <- r:
	default:
		c.Log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		_ = c.Close()
	}
}

func (c *conn) Reply(id, reply []byte) {
	if c.isClosed() {
		return
	}
	l := len(reply)
	// 7 = size(4 Bytes) + NodeReply(1 byte) + msg id(2 bytes)
	b := make([]byte, protocol.FrameHeaderSize+l)
	// write size
	msgSize := protocol.FrameCMDSize + protocol.FrameIDSize + l
	copy(b, convert.Uint32ToBytes(uint32(msgSize)))
	// write cmd
	b[protocol.FrameLenSize] = protocol.ConnReply
	// write msg id
	copy(b[protocol.FrameLenSize+1:protocol.FrameLenSize+1+protocol.FrameIDSize], id)
	// write data
	copy(b[protocol.FrameHeaderSize:], reply)
	_ = c.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = c.Write(b)
}

func (c *conn) HandleHeartbeat() {
	// <security> fake traffic like client
	fakeSize := 64 + c.rand.Int(256)
	// size(4 Bytes) + heartbeat(1 byte) + fake data
	c.heartbeat.Reset()
	c.heartbeat.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
	c.heartbeat.WriteByte(protocol.ConnReplyHeartbeat)
	c.heartbeat.Write(c.rand.Bytes(fakeSize))
	// send heartbeat data
	_ = c.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = c.Write(c.heartbeat.Bytes())
}

func (c *conn) HandleSendToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid send to node guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckSendToNodeGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleSendToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid send to beacon guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckSendToBeaconGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleAckToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid ack to node guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckAckToNodeGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleAckToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid ack to beacon guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckAckToBeaconGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBroadcastGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid broadcast guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckBroadcastGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleAnswerGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid answer guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckAnswerGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleNodeSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid node send guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckNodeSendGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleNodeAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid node ack guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckNodeAckGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBeaconSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid beacon send guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckBeaconSendGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBeaconAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid beacon ack guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckBeaconAckGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBeaconQueryGUID(id, data []byte) {
	if len(data) != guid.Size {
		c.Log(logger.Exploit, "invalid query guid size")
		c.Reply(id, protocol.ReplyHandled)
		_ = c.Close()
		return
	}
	if c.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckQueryGUIDSlice(data) {
		c.Reply(id, protocol.ReplyUnhandled)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleSendToNode(id, data []byte) {
	send := c.ctx.worker.GetSendFromPool()
	err := send.Unpack(data)
	if err != nil {
		const format = "invalid send to node data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		c.ctx.worker.PutSendToPool(send)
		_ = c.Close()
		return
	}
	err = send.Validate()
	if err != nil {
		const format = "invalid send to node: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		c.ctx.worker.PutSendToPool(send)
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&send.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		c.ctx.worker.PutSendToPool(send)
		return
	}
	if c.ctx.syncer.CheckSendToNodeGUID(&send.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		if send.RoleGUID == *c.ctx.global.GUID() {
			c.ctx.worker.AddSend(send)
		} else {
			c.ctx.forwarder.SendToNode(&send.GUID, data, c.tag)
			c.ctx.worker.PutSendToPool(send)
		}
	} else {
		c.Reply(id, protocol.ReplyHandled)
		c.ctx.worker.PutSendToPool(send)
	}
}

func (c *conn) HandleAckToNode(id, data []byte) {
	ack := c.ctx.worker.GetAcknowledgeFromPool()
	err := ack.Unpack(data)
	if err != nil {
		const format = "invalid ack to node data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		c.ctx.worker.PutAcknowledgeToPool(ack)
		_ = c.Close()
		return
	}
	err = ack.Validate()
	if err != nil {
		const format = "invalid ack to node: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		c.ctx.worker.PutAcknowledgeToPool(ack)
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&ack.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		c.ctx.worker.PutAcknowledgeToPool(ack)
		return
	}
	if c.ctx.syncer.CheckAckToNodeGUID(&ack.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		if ack.RoleGUID == *c.ctx.global.GUID() {
			c.ctx.worker.AddAcknowledge(ack)
		} else {
			c.ctx.forwarder.AckToNode(&ack.GUID, data, c.tag)
			c.ctx.worker.PutAcknowledgeToPool(ack)
		}
	} else {
		c.Reply(id, protocol.ReplyHandled)
		c.ctx.worker.PutAcknowledgeToPool(ack)
	}
}

func (c *conn) HandleSendToBeacon(id, data []byte) {
	send := c.SendPool.Get().(*protocol.Send)
	defer c.SendPool.Put(send)
	err := send.Unpack(data)
	if err != nil {
		const format = "invalid send to node data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		_ = c.Close()
		return
	}
	err = send.Validate()
	if err != nil {
		const format = "invalid send to beacon: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&send.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckSendToBeaconGUID(&send.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleAckToBeacon(id, data []byte) {
	ack := c.AckPool.Get().(*protocol.Acknowledge)
	defer c.AckPool.Put(ack)
	err := ack.Unpack(data)
	if err != nil {
		const format = "invalid ack to beacon data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		_ = c.Close()
		return
	}
	err = ack.Validate()
	if err != nil {
		const format = "invalid ack to beacon: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&ack.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckAckToBeaconGUID(&ack.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBroadcast(id, data []byte) {
	broadcast := c.ctx.worker.GetBroadcastFromPool()
	err := broadcast.Unpack(data)
	if err != nil {
		const format = "invalid broadcast data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(broadcast))
		c.ctx.worker.PutBroadcastToPool(broadcast)
		_ = c.Close()
		return
	}
	err = broadcast.Validate()
	if err != nil {
		const format = "invalid broadcast: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(broadcast))
		c.ctx.worker.PutBroadcastToPool(broadcast)
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&broadcast.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		c.ctx.worker.PutBroadcastToPool(broadcast)
		return
	}
	if c.ctx.syncer.CheckBroadcastGUID(&broadcast.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		c.ctx.worker.AddBroadcast(broadcast)
		c.ctx.forwarder.Broadcast(&broadcast.GUID, data, c.tag)
	} else {
		c.Reply(id, protocol.ReplyHandled)
		c.ctx.worker.PutBroadcastToPool(broadcast)
	}
}

func (c *conn) HandleAnswer(id, data []byte) {
	answer := c.AnswerPool.Get().(*protocol.Answer)
	defer c.AnswerPool.Put(answer)
	err := answer.Unpack(data)
	if err != nil {
		const format = "invalid answer data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(answer))
		_ = c.Close()
		return
	}
	err = answer.Validate()
	if err != nil {
		const format = "invalid answer: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(answer))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&answer.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckAnswerGUID(&answer.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleNodeSend(id, data []byte) {
	send := c.SendPool.Get().(*protocol.Send)
	defer c.SendPool.Put(send)
	err := send.Unpack(data)
	if err != nil {
		const format = "invalid node send data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		_ = c.Close()
		return
	}
	err = send.Validate()
	if err != nil {
		const format = "invalid node send: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&send.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckNodeSendGUID(&send.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		c.ctx.forwarder.NodeSend(&send.GUID, data, c.tag)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleNodeAck(id, data []byte) {
	ack := c.AckPool.Get().(*protocol.Acknowledge)
	defer c.AckPool.Put(ack)
	err := ack.Unpack(data)
	if err != nil {
		const format = "invalid node ack data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		_ = c.Close()
		return
	}
	err = ack.Validate()
	if err != nil {
		const format = "invalid node ack: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&ack.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckNodeAckGUID(&ack.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
		c.ctx.forwarder.NodeAck(&ack.GUID, data, c.tag)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBeaconSend(id, data []byte) {
	send := c.SendPool.Get().(*protocol.Send)
	defer c.SendPool.Put(send)
	err := send.Unpack(data)
	if err != nil {
		const format = "invalid beacon send data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		_ = c.Close()
		return
	}
	err = send.Validate()
	if err != nil {
		const format = "invalid beacon send: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(send))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&send.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckBeaconSendGUID(&send.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBeaconAck(id, data []byte) {
	ack := c.AckPool.Get().(*protocol.Acknowledge)
	defer c.AckPool.Put(ack)
	err := ack.Unpack(data)
	if err != nil {
		const format = "invalid beacon ack data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		_ = c.Close()
		return
	}
	err = ack.Validate()
	if err != nil {
		const format = "invalid beacon ack: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(ack))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&ack.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckBeaconAckGUID(&ack.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

func (c *conn) HandleBeaconQuery(id, data []byte) {
	query := c.QueryPool.Get().(*protocol.Query)
	defer c.QueryPool.Put(query)
	err := query.Unpack(data)
	if err != nil {
		const format = "invalid query data: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(query))
		_ = c.Close()
		return
	}
	err = query.Validate()
	if err != nil {
		const format = "invalid query: %s\n%s"
		c.Logf(logger.Exploit, format, err, spew.Sdump(query))
		_ = c.Close()
		return
	}
	expired, timestamp := c.ctx.syncer.CheckGUIDTimestamp(&query.GUID)
	if expired {
		c.Reply(id, protocol.ReplyExpired)
		return
	}
	if c.ctx.syncer.CheckQueryGUID(&query.GUID, timestamp) {
		c.Reply(id, protocol.ReplySucceed)
	} else {
		c.Reply(id, protocol.ReplyHandled)
	}
}

// send is used to send command and receive reply
func (c *conn) send(cmd uint8, data []byte) ([]byte, error) {
	if c.isClosed() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-c.slots[id].Available:
				l := len(data)
				b := make([]byte, protocol.FrameHeaderSize+l)
				// write MsgLen
				msgSize := protocol.FrameCMDSize + protocol.FrameIDSize + l
				copy(b, convert.Uint32ToBytes(uint32(msgSize)))
				// write cmd
				b[protocol.FrameLenSize] = cmd
				// write msg id
				copy(b[protocol.FrameLenSize+1:protocol.FrameLenSize+1+protocol.FrameIDSize],
					convert.Uint16ToBytes(uint16(id)))
				// write data
				copy(b[protocol.FrameHeaderSize:], data)
				// send
				_ = c.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := c.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				if !c.slots[id].Timer.Stop() {
					<-c.slots[id].Timer.C
				}
				c.slots[id].Timer.Reset(protocol.RecvTimeout)
				select {
				case r := <-c.slots[id].Reply:
					c.slots[id].Available <- struct{}{}
					return r, nil
				case <-c.slots[id].Timer.C:
					_ = c.Close()
					return nil, protocol.ErrRecvReplyTimeout
				case <-c.stopSignal:
					return nil, protocol.ErrConnClosed
				}
			case <-c.stopSignal:
				return nil, protocol.ErrConnClosed
			default:
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-c.stopSignal:
			return nil, protocol.ErrConnClosed
		}
	}
}

func (c *conn) SendMessage(msg []byte) error {
	return c.Conn.Send(msg)
}

func (c *conn) SendCommand(cmd uint8, data []byte) ([]byte, error) {
	return c.send(cmd, data)
}

// Send is used to send message to Controller
func (c *conn) Send(guid *guid.GUID, data *bytes.Buffer) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: c.role,
		GUID: c.guid,
	}
	var reply []byte
	reply, sr.Err = c.send(protocol.NodeSendGUID, guid[:])
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = c.send(protocol.NodeSend, data.Bytes())
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		sr.Err = errors.New(string(reply))
	}
	return
}

// Acknowledge is used to notice Controller that Node has received this message
func (c *conn) Acknowledge(guid *guid.GUID, data *bytes.Buffer) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: c.role,
		GUID: c.guid,
	}
	var reply []byte
	reply, ar.Err = c.send(protocol.NodeAckGUID, guid[:])
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		ar.Err = protocol.GetReplyError(reply)
		return
	}
	reply, ar.Err = c.send(protocol.NodeAck, data.Bytes())
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		ar.Err = errors.New(string(reply))
	}
	return
}

// -------------------------------------------forwarder----------------------------------------------

// Broadcast is used to forward Controller broadcast message to Nodes
func (c *conn) Broadcast(guid, data []byte) {
	reply, err := c.send(protocol.CtrlBroadcastGUID, guid)
	if err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	_, _ = c.send(protocol.CtrlBroadcast, data)
}

// SendToNode is used to forward Controller send message to Node
func (c *conn) SendToNode(guid, data []byte) {
	reply, err := c.send(protocol.CtrlSendToNodeGUID, guid)
	if err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	_, _ = c.send(protocol.CtrlSendToNode, data)
}

// AckToNode is used to forward Controller acknowledge to Node
func (c *conn) AckToNode(guid, data []byte) {
	reply, err := c.send(protocol.CtrlAckToNodeGUID, guid)
	if err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	_, _ = c.send(protocol.CtrlAckToNode, data)
}

// NodeSend is used to forward Node send
func (c *conn) NodeSend(guid, data []byte) {
	reply, err := c.send(protocol.NodeSendGUID, guid)
	if err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	_, _ = c.send(protocol.NodeSend, data)
}

// NodeAck is used to forward Node acknowledge
func (c *conn) NodeAck(guid, data []byte) {
	reply, err := c.send(protocol.NodeAckGUID, guid)
	if err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	_, _ = c.send(protocol.NodeAck, data)
}

func (c *conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		atomic.StoreInt32(&c.inClose, 1)
		close(c.stopSignal)
		err = c.Conn.Close()
	})
	return err
}

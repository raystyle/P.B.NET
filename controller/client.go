package controller

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
	"project/internal/xpanic"
)

type client struct {
	ctx *CTRL

	node      *bootstrap.Node
	guid      []byte // node guid
	closeFunc func()

	conn      *xnet.Conn
	slots     []*protocol.Slot
	heartbeat chan struct{}
	sync      int32

	closing    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// guid == nil        for trust node
// guid != nil        for sender client
// guid == ctrl guid, for discovery
func newClient(ctx *CTRL, node *bootstrap.Node, guid []byte, closeFunc func()) (*client, error) {
	xnetCfg := xnet.Config{
		Network: node.Network,
		Address: node.Address,
	}
	xnetCfg.TLSConfig.RootCAs = ctx.global.CACertificatesStr()
	conn, err := xnet.Dial(node.Mode, &xnetCfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	client := client{
		ctx:       ctx,
		node:      node,
		guid:      guid,
		closeFunc: closeFunc,
	}
	xconn, err := client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessage(err, "handshake failed")
	}
	client.conn = xconn
	// init slot
	client.slots = make([]*protocol.Slot, protocol.SlotSize)
	for i := 0; i < protocol.SlotSize; i++ {
		s := &protocol.Slot{
			Available: make(chan struct{}, 1),
			Reply:     make(chan []byte, 1),
			Timer:     time.NewTimer(protocol.RecvTimeout),
		}
		s.Available <- struct{}{}
		client.slots[i] = s
	}
	client.heartbeat = make(chan struct{}, 1)
	client.stopSignal = make(chan struct{})
	// <warning> not add wg
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := xpanic.Error("client panic:", r)
				client.log(logger.Fatal, err)
			}
			client.Close()
		}()
		protocol.HandleConn(client.conn, client.handleMessage)
	}()
	client.wg.Add(1)
	go client.sendHeartbeatLoop()
	return &client, nil
}

func (client *client) isClosing() bool {
	return atomic.LoadInt32(&client.closing) != 0
}

func (client *client) Reply(id, reply []byte) {
	if client.isClosing() {
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
	_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = client.conn.Write(b)
}

// send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
// data(general) max size = MaxMsgSize -MsgCMDSize -MsgIDSize
func (client *client) Send(cmd uint8, data []byte) ([]byte, error) {
	if client.isClosing() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-client.slots[id].Available:
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
				_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := client.conn.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				if !client.slots[id].Timer.Stop() {
					<-client.slots[id].Timer.C
				}
				client.slots[id].Timer.Reset(protocol.RecvTimeout)
				select {
				case r := <-client.slots[id].Reply:
					client.slots[id].Available <- struct{}{}
					return r, nil
				case <-client.slots[id].Timer.C:
					client.Close()
					return nil, protocol.ErrRecvTimeout
				case <-client.stopSignal:
					return nil, protocol.ErrConnClosed
				}
			case <-client.stopSignal:
				return nil, protocol.ErrConnClosed
			default:
				// try next slot
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-client.stopSignal:
			return nil, protocol.ErrConnClosed
		}
	}
}

func (client *client) isSync() bool {
	return atomic.LoadInt32(&client.sync) != 0
}

func (client *client) Sync() error {
	resp, err := client.Send(protocol.CtrlSync, nil)
	if err != nil {
		return errors.Wrap(err, "receive sync response failed")
	}
	if !bytes.Equal(resp, []byte{protocol.NodeSync}) {
		return errors.Errorf("sync failed: %s", string(resp))
	}
	atomic.StoreInt32(&client.sync, 1)
	return nil
}

// TODO error
func (client *client) Broadcast(guid, data []byte) (br *protocol.BroadcastResponse) {
	br = &protocol.BroadcastResponse{
		GUID: client.guid,
	}
	var reply []byte
	reply, br.Err = client.Send(protocol.CtrlBroadcastGUID, guid)
	if br.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.BroadcastUnhandled) {
		br.Err = errors.New(string(reply))
		return
	}
	// broadcast
	reply, br.Err = client.Send(protocol.CtrlBroadcast, data)
	if br.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.BroadcastSucceed) {
		br.Err = errors.New(string(reply))
	}
	return
}

func (client *client) SendToNode(guid, data []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, sr.Err = client.Send(protocol.CtrlSendToNodeGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		sr.Err = errors.New(string(reply))
		return
	}
	reply, sr.Err = client.Send(protocol.CtrlSendToNode, data)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

func (client *client) SendToBeacon(guid, data []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, sr.Err = client.Send(protocol.CtrlSendToBeaconGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		sr.Err = errors.New(string(reply))
		return
	}
	reply, sr.Err = client.Send(protocol.CtrlSendToBeacon, data)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

// AcknowledgeToNode is used to notice Node that
// Controller has received this message
func (client *client) AcknowledgeToNode(guid, data []byte) {
	var (
		reply []byte
		err   error
	)
	defer func() {
		if err != nil {
			client.logln(logger.Error, "acknowledge to node failed:", err)
		}
	}()
	reply, err = client.Send(protocol.CtrlAckToNodeGUID, guid)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		return
	}
	reply, err = client.Send(protocol.CtrlAckToNode, data)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		err = errors.New(string(reply))
	}
}

// AcknowledgeToBeacon is used to notice Beacon that
// Controller has received this message
func (client *client) AcknowledgeToBeacon(guid, data []byte) {
	var (
		reply []byte
		err   error
	)
	defer func() {
		if err != nil {
			client.logln(logger.Error, "acknowledge to beacon failed:", err)
		}
	}()
	reply, err = client.Send(protocol.CtrlAckToBeaconGUID, guid)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		return
	}
	reply, err = client.Send(protocol.CtrlAckToBeacon, data)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		err = errors.New(string(reply))
	}
}

func (client *client) Status() *xnet.Status {
	return client.conn.Status()
}

func (client *client) Close() {
	client.closeOnce.Do(func() {
		atomic.StoreInt32(&client.closing, 1)
		_ = client.conn.Close()
		close(client.stopSignal)
		client.wg.Wait()
		if client.closeFunc != nil {
			client.closeFunc()
		}
		client.log(logger.Info, "disconnected")
	})
}

func (client *client) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(client.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	client.ctx.logger.Print(l, "client", b)
}

func (client *client) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(client.conn)
	_, _ = fmt.Fprint(b, log...)
	client.ctx.logger.Print(l, "client", b)
}

func (client *client) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(client.conn)
	_, _ = fmt.Fprintln(b, log...)
	client.ctx.logger.Print(l, "client", b)
}

func (client *client) handshake(conn net.Conn) (*xnet.Conn, error) {
	dConn := xnet.NewDeadlineConn(conn, time.Minute)
	xConn := xnet.NewConn(dConn, client.ctx.global.Now())
	// receive certificate
	cert, err := xConn.Receive()
	if err != nil {
		return nil, errors.Wrap(err, "receive certificate failed")
	}
	if !client.ctx.verifyCertificate(cert, client.node.Address, client.guid) {
		client.log(logger.Exploit, protocol.ErrInvalidCert)
		return nil, protocol.ErrInvalidCert
	}
	// send role
	_, err = xConn.Write(protocol.Ctrl.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "send role failed")
	}
	// receive challenge
	challenge, err := xConn.Receive()
	if err != nil {
		return nil, errors.Wrap(err, "receive challenge data failed")
	}
	// <danger>
	// receive random challenge data(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and if controller sign it will destroy net
	if len(challenge) < 2048 || len(challenge) > 4096 {
		err = errors.New("invalid challenge size")
		client.log(logger.Exploit, err)
		return nil, err
	}
	// send signature
	err = xConn.Send(client.ctx.global.Sign(challenge))
	if err != nil {
		return nil, errors.Wrap(err, "send challenge signature failed")
	}
	resp, err := xConn.Receive()
	if err != nil {
		return nil, errors.Wrap(err, "receive authentication response failed")
	}
	if !bytes.Equal(resp, protocol.AuthSucceed) {
		err = errors.WithStack(protocol.ErrAuthFailed)
		client.log(logger.Exploit, err)
		return nil, err
	}
	// remove deadline conn
	return xnet.NewConn(conn, client.ctx.global.Now()), nil
}

func (client *client) sendHeartbeatLoop() {
	defer client.wg.Done()
	var err error
	rand := random.New(0)
	buffer := bytes.NewBuffer(nil)
	for {
		t := time.Duration(30+rand.Int(60)) * time.Second
		select {
		case <-time.After(t):
			// <security> fake traffic like client
			fakeSize := 64 + rand.Int(256)
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			buffer.Reset()
			buffer.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
			buffer.WriteByte(protocol.CtrlHeartbeat)
			buffer.Write(rand.Bytes(fakeSize))
			// send
			_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err = client.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			select {
			case <-client.heartbeat:
			case <-time.After(t):
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

// can use client.Close()
func (client *client) handleMessage(msg []byte) {
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if client.isClosing() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(msg) < id {
		client.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		client.Close()
		return
	}
	if client.isSync() {
		switch msg[0] {
		case protocol.BeaconQueryGUID:
			client.handleBeaconQueryGUID(msg[cmd:id], msg[id:])
		case protocol.BeaconQuery:
			client.handleBeaconQuery(msg[cmd:id], msg[id:])
		case protocol.BeaconSendGUID:
			client.handleBeaconSendGUID(msg[cmd:id], msg[id:])
		case protocol.BeaconSend:
			client.handleBeaconSend(msg[cmd:id], msg[id:])
		case protocol.NodeSendGUID:
			client.handleNodeSendGUID(msg[cmd:id], msg[id:])
		case protocol.NodeSend:
			client.handleNodeSend(msg[cmd:id], msg[id:])
		}
	}
	switch msg[0] {
	case protocol.NodeReply:
		client.handleReply(msg[cmd:])
	case protocol.NodeHeartbeat:
		select {
		case client.heartbeat <- struct{}{}:
		case <-client.stopSignal:
		}
	case protocol.ErrCMDRecvNullMsg:
		client.log(logger.Exploit, protocol.ErrRecvNullMsg)
		client.Close()
	case protocol.ErrCMDTooBigMsg:
		client.log(logger.Exploit, protocol.ErrRecvTooBigMsg)
		client.Close()
	case protocol.TestCommand:
		client.Reply(msg[cmd:id], msg[id:])
	default:
		client.log(logger.Exploit, protocol.ErrRecvUnknownCMD, msg)
		client.Close()
	}
}

// msg id(2 bytes) + data
func (client *client) handleReply(reply []byte) {
	l := len(reply)
	if l < protocol.MsgIDSize {
		client.log(logger.Exploit, protocol.ErrRecvInvalidMsgIDSize)
		client.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:protocol.MsgIDSize]))
	if id > protocol.MaxMsgID {
		client.log(logger.Exploit, protocol.ErrRecvInvalidMsgID)
		client.Close()
		return
	}
	// must copy
	r := make([]byte, l-protocol.MsgIDSize)
	copy(r, reply[protocol.MsgIDSize:])
	// <security> maybe wrong msg id
	select {
	case client.slots[id].Reply <- r:
	default:
		client.log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		client.Close()
	}
}

func (client *client) handleNodeSendGUID(id, guid_ []byte) {
	if len(guid_) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid node send guid size")
		client.Reply(id, protocol.SendHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(guid_); expired {
		client.Reply(id, protocol.SendExpired)
	} else if client.ctx.syncer.CheckNodeSendGUID(guid_, false, 0) {
		client.Reply(id, protocol.SendUnhandled)
	} else {
		client.Reply(id, protocol.SendHandled)
	}
}

func (client *client) handleBeaconSendGUID(id, guid_ []byte) {
	if len(guid_) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid beacon send guid size")
		client.Reply(id, protocol.SendHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(guid_); expired {
		client.Reply(id, protocol.SendExpired)
	} else if client.ctx.syncer.CheckBeaconSendGUID(guid_, false, 0) {
		client.Reply(id, protocol.SendUnhandled)
	} else {
		client.Reply(id, protocol.SendHandled)
	}
}

func (client *client) handleBeaconQueryGUID(id, guid_ []byte) {
	if len(guid_) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid beacon query guid size")
		client.Reply(id, protocol.SendHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(guid_); expired {
		client.Reply(id, protocol.SendExpired)
	} else if client.ctx.syncer.CheckBeaconQueryGUID(guid_, false, 0) {
		client.Reply(id, protocol.SendUnhandled)
	} else {
		client.Reply(id, protocol.SendHandled)
	}
}

func (client *client) handleNodeSend(id, data []byte) {
	s := protocol.Send{}
	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		client.logln(logger.Exploit, "invalid node send msgpack data:", err)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid node send: %s\n%s", err, spew.Sdump(s))
		client.Close()
		return
	}
	if expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID); expired {
		client.Reply(id, protocol.SendExpired)
	} else if client.ctx.syncer.CheckNodeSendGUID(s.GUID, true, timestamp) {
		client.Reply(id, protocol.SendSucceed)
		client.ctx.syncer.AddNodeSend(&s)
	} else {
		client.Reply(id, protocol.SendHandled)
	}
}

func (client *client) handleBeaconSend(id, data []byte) {
	s := protocol.Send{}
	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		client.logln(logger.Exploit, "invalid beacon send msgpack data:", err)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon send: %s\n%s", err, spew.Sdump(s))
		client.Close()
		return
	}
	if expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID); expired {
		client.Reply(id, protocol.SendExpired)
	} else if client.ctx.syncer.CheckBeaconSendGUID(s.GUID, true, timestamp) {
		client.Reply(id, protocol.SendSucceed)
		client.ctx.syncer.AddBeaconSend(&s)
	} else {
		client.Reply(id, protocol.SendHandled)
	}
}

func (client *client) handleBeaconQuery(id, data []byte) {
	q := protocol.Query{}
	err := msgpack.Unmarshal(data, &q)
	if err != nil {
		client.logln(logger.Exploit, "invalid beacon query msgpack data:", err)
		client.Close()
		return
	}
	err = q.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon query: %s\n%s", err, spew.Sdump(q))
		client.Close()
		return
	}
	if expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(q.GUID); expired {
		client.Reply(id, protocol.SendExpired)
	} else if client.ctx.syncer.CheckBeaconQueryGUID(q.GUID, true, timestamp) {
		client.Reply(id, protocol.SendSucceed)
		client.ctx.syncer.AddBeaconQuery(&q)
	} else {
		client.Reply(id, protocol.SendHandled)
	}
}

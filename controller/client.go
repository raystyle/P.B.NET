package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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
	"project/internal/crypto/rand"
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
	inSync    int32

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// when guid == nil        for trust node
// when guid != nil        for sender client
// when guid == ctrl guid, for discovery
func newClient(
	ctrl *CTRL,
	ctx context.Context,
	node *bootstrap.Node,
	guid []byte,
	closeFunc func(),
) (*client, error) {

	cfg := xnet.Config{
		Network: node.Network,
		Timeout: ctrl.opts.Timeout,
	}

	cfg.TLSConfig = &tls.Config{
		Rand:       rand.Reader,
		Time:       ctrl.global.Now,
		ServerName: node.Address,
		RootCAs:    x509.NewCertPool(),
		MinVersion: tls.VersionTLS12,
	}

	// add CA certificates
	for _, cert := range ctrl.global.GetSystemCA() {
		cfg.TLSConfig.RootCAs.AddCert(cert)
	}
	for _, kp := range ctrl.global.GetSelfCA() {
		cfg.TLSConfig.RootCAs.AddCert(kp.Certificate)
	}

	// set proxy
	p, _ := ctrl.global.GetProxyClient(ctrl.opts.ProxyTag)
	cfg.Dialer = p.DialContext

	// resolve domain name
	host, port, err := net.SplitHostPort(node.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	result, err := ctrl.global.ResolveWithContext(ctx, host, &ctrl.opts.DNSOpts)
	if err != nil {
		return nil, err
	}
	var conn *xnet.Conn
	for i := 0; i < len(result); i++ {
		cfg.Address = net.JoinHostPort(result[i], port)
		c, err := xnet.DialContext(ctx, node.Mode, &cfg)
		if err == nil {
			conn = xnet.NewConn(c, ctrl.global.Now())
			break
		}
	}
	if conn == nil {
		return nil, errors.Errorf("failed to connect node: %s", node.Address)
	}

	// handshake
	client := client{
		ctx:       ctrl,
		node:      node,
		guid:      guid,
		closeFunc: closeFunc,
	}
	err = client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		const format = "failed to handshake with node: %s"
		return nil, errors.WithMessagef(err, format, node.Address)
	}
	client.conn = conn

	// initialize message slots
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
				client.log(logger.Fatal, xpanic.Error(r, "client:"))
			}
			client.Close()
		}()
		protocol.HandleConn(client.conn, client.onFrame)
	}()
	client.wg.Add(1)
	go client.sendHeartbeatLoop()
	return &client, nil
}

func (client *client) log(l logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprint(b, log...)
	_, _ = fmt.Fprint(b, "\n", client.conn)
	client.ctx.logger.Print(l, "client", b)
}

func (client *client) logf(l logger.Level, format string, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, format, log...)
	_, _ = fmt.Fprint(b, "\n", client.conn)
	client.ctx.logger.Print(l, "client", b)
}

// Zero—Knowledge Proof
func (client *client) handshake(conn *xnet.Conn) error {
	_ = conn.SetDeadline(client.ctx.global.Now().Add(client.ctx.opts.Timeout))
	// receive certificate
	cert, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive certificate")
	}
	if !client.ctx.verifyCertificate(cert, client.node.Address, client.guid) {
		client.log(logger.Exploit, protocol.ErrInvalidCertificate)
		return protocol.ErrInvalidCertificate
	}
	// send role
	_, err = conn.Write(protocol.Ctrl.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send role")
	}
	// receive challenge
	challenge, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive challenge")
	}
	// <danger>
	// receive random challenge data(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and if controller sign it will destroy net
	if len(challenge) < 2048 || len(challenge) > 4096 {
		err = errors.New("invalid challenge size")
		client.log(logger.Exploit, err)
		return err
	}
	// send signature
	err = conn.Send(client.ctx.global.Sign(challenge))
	if err != nil {
		return errors.Wrap(err, "failed to send challenge signature")
	}
	resp, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive authentication response")
	}
	if !bytes.Equal(resp, protocol.AuthSucceed) {
		err = errors.WithStack(protocol.ErrAuthenticateFailed)
		client.log(logger.Exploit, err)
		return err
	}
	return conn.SetDeadline(time.Time{})
}

func (client *client) isSync() bool {
	return atomic.LoadInt32(&client.inSync) != 0
}

func (client *client) isClosing() bool {
	return atomic.LoadInt32(&client.inClose) != 0
}

// can use client.Close()
func (client *client) onFrame(frame []byte) {
	if client.isClosing() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(frame) < protocol.MsgCMDSize+protocol.MsgIDSize {
		client.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		client.Close()
		return
	}
	id := frame[protocol.MsgCMDSize : protocol.MsgCMDSize+protocol.MsgIDSize]
	data := frame[protocol.MsgCMDSize+protocol.MsgIDSize:]
	if client.isSync() {
		switch frame[0] {
		case protocol.NodeSendGUID:
			client.handleNodeSendGUID(id, data)
		case protocol.NodeSend:
			client.handleNodeSend(id, data)
		case protocol.NodeAckGUID:
			client.handleNodeAckGUID(id, data)
		case protocol.NodeAck:

		case protocol.BeaconQueryGUID:
			client.handleBeaconQueryGUID(id, data)
		case protocol.BeaconQuery:
			client.handleBeaconQuery(id, data)
		case protocol.BeaconSendGUID:
			client.handleBeaconSendGUID(id, data)
		case protocol.BeaconSend:
			client.handleBeaconSend(id, data)
		case protocol.BeaconAckGUID:
			client.handleBeaconAckGUID(id, data)
		case protocol.BeaconAck:

		}
	}
	switch frame[0] {
	case protocol.ConnReply:
		client.handleReply(frame[protocol.MsgCMDSize:])
	case protocol.ConnReplyHeartbeat:
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
	default:
		client.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		client.Close()
	}
}

func (client *client) sendHeartbeatLoop() {
	defer client.wg.Done()
	var err error
	r := random.New(client.ctx.global.Now().Unix())
	buffer := bytes.NewBuffer(nil)
	for {
		t := time.Duration(30+r.Int(60)) * time.Second
		select {
		case <-time.After(t):
			// <security> fake traffic like client
			fakeSize := 64 + r.Int(256)
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			buffer.Reset()
			buffer.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
			buffer.WriteByte(protocol.ConnSendHeartbeat)
			buffer.Write(r.Bytes(fakeSize))
			// send
			_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err = client.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			// receive reply
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
	// <security> maybe incorrect msg id
	select {
	case client.slots[id].Reply <- r:
	default:
		client.log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		client.Close()
	}
}

func (client *client) handleNodeSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid node send guid size")
		client.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckNodeSendGUID(data, false, 0) {
		client.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid node acknowledge guid size")
		client.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckNodeAckGUID(data, false, 0) {
		client.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid beacon send guid size")
		client.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckBeaconSendGUID(data, false, 0) {
		client.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid beacon acknowledge guid size")
		client.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckBeaconAckGUID(data, false, 0) {
		client.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconQueryGUID(id, data []byte) {
	if len(data) != guid.Size {
		// fake reply and close
		client.log(logger.Exploit, "invalid beacon query guid size")
		client.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckQueryGUID(data, false, 0) {
		client.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeSend(id, data []byte) {
	s := client.ctx.worker.GetSendFromPool()
	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		client.log(logger.Exploit, "invalid node send msgpack data: ", err)
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid node send: %s\n%s", err, spew.Sdump(s))
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		client.Reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutSendToPool(s)
		return
	}
	if client.ctx.syncer.CheckNodeSendGUID(s.GUID, true, timestamp) {
		client.Reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddNodeSend(s)
	} else {
		client.Reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(s)
	}
}

func (client *client) handleBeaconSend(id, data []byte) {
	s := client.ctx.worker.GetSendFromPool()
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		client.log(logger.Exploit, "invalid beacon send msgpack data: ", err)
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon send: %s\n%s", err, spew.Sdump(s))
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		client.Reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutSendToPool(s)
		return
	}
	if client.ctx.syncer.CheckBeaconSendGUID(s.GUID, true, timestamp) {
		client.Reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddBeaconSend(s)
	} else {
		client.Reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(s)
	}
}

func (client *client) handleBeaconQuery(id, data []byte) {
	q := client.ctx.worker.GetQueryFromPool()
	err := msgpack.Unmarshal(data, q)
	if err != nil {
		client.log(logger.Exploit, "invalid beacon query msgpack data: ", err)
		client.ctx.worker.PutQueryToPool(q)
		client.Close()
		return
	}
	err = q.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon query: %s\n%s", err, spew.Sdump(q))
		client.ctx.worker.PutQueryToPool(q)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(q.GUID)
	if expired {
		client.Reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutQueryToPool(q)
		return
	}
	if client.ctx.syncer.CheckQueryGUID(q.GUID, true, timestamp) {
		client.Reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddQuery(q)
	} else {
		client.Reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutQueryToPool(q)
	}
}

// Reply is used to reply command
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
	b[protocol.MsgLenSize] = protocol.ConnReply
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

// Sync is used to switch to sync mode
func (client *client) Sync() error {
	resp, err := client.Send(protocol.CtrlSync, nil)
	if err != nil {
		return errors.Wrap(err, "receive sync response failed")
	}
	if !bytes.Equal(resp, []byte{protocol.NodeSync}) {
		return errors.Errorf("sync failed: %s", string(resp))
	}
	atomic.StoreInt32(&client.inSync, 1)
	return nil
}

// Broadcast is used to broadcast message to nodes
func (client *client) Broadcast(guid, data []byte) (br *protocol.BroadcastResponse) {
	br = &protocol.BroadcastResponse{
		GUID: client.guid,
	}
	var reply []byte
	reply, br.Err = client.Send(protocol.CtrlBroadcastGUID, guid)
	if br.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		br.Err = protocol.GetReplyError(reply)
		return
	}
	// broadcast
	reply, br.Err = client.Send(protocol.CtrlBroadcast, data)
	if br.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		br.Err = protocol.GetReplyError(reply)
	}
	return
}

// SendToNode is used to send message to node
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
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = client.Send(protocol.CtrlSendToNode, data)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		sr.Err = protocol.GetReplyError(reply)
	}
	return
}

// SendToBeacon is used to send message to beacon
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
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = client.Send(protocol.CtrlSendToBeacon, data)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		sr.Err = protocol.GetReplyError(reply)
	}
	return
}

// AcknowledgeToNode is used to notice Node that
// Controller has received this message
func (client *client) AcknowledgeToNode(guid, data []byte) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.Send(protocol.CtrlAckToNodeGUID, guid)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		return
	}
	reply, ar.Err = client.Send(protocol.CtrlAckToNode, data)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		ar.Err = errors.New(string(reply))
	}
	return
}

// AcknowledgeToBeacon is used to notice Beacon that
// Controller has received this message
func (client *client) AcknowledgeToBeacon(guid, data []byte) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.Send(protocol.CtrlAckToBeaconGUID, guid)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		return
	}
	reply, ar.Err = client.Send(protocol.CtrlAckToBeacon, data)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		ar.Err = errors.New(string(reply))
	}
	return
}

// Answer is used to return the result of the beacon query
func (client *client) Answer(guid, data []byte) (ar *protocol.AnswerResponse) {
	ar = &protocol.AnswerResponse{
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.Send(protocol.CtrlAnswerGUID, guid)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		return
	}
	reply, ar.Err = client.Send(protocol.CtrlAnswer, data)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		ar.Err = errors.New(string(reply))
	}
	return
}

// Status is used to get connection status
func (client *client) Status() *xnet.Status {
	return client.conn.Status()
}

// Close is used to disconnect node
func (client *client) Close() {
	client.closeOnce.Do(func() {
		atomic.StoreInt32(&client.inClose, 1)
		_ = client.conn.Close()
		close(client.stopSignal)
		client.wg.Wait()
		if client.closeFunc != nil {
			client.closeFunc()
		}
		client.log(logger.Info, "disconnected")
	})
}

package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/ed25519"
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
	syncM     sync.Mutex

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// when guid == nil       for trust node
// when guid != ctrl guid for sender client
// when guid == ctrl guid for discovery
func newClient(
	ctx context.Context,
	ctrl *CTRL,
	node *bootstrap.Node,
	guid []byte,
	closeFunc func(),
) (*client, error) {
	host, port, err := net.SplitHostPort(node.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cfg := xnet.Config{
		Network: node.Network,
		Timeout: ctrl.client.Timeout,
	}
	cfg.TLSConfig = &tls.Config{
		Rand:       rand.Reader,
		Time:       ctrl.global.Now,
		ServerName: host,
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
	p, _ := ctrl.global.GetProxyClient(ctrl.client.ProxyTag)
	cfg.Dialer = p.DialContext
	// resolve domain name
	result, err := ctrl.global.ResolveWithContext(ctx, host, &ctrl.client.DNSOpts)
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
		client.slots[i] = protocol.NewSlot()
	}
	client.stopSignal = make(chan struct{})
	// heartbeat
	client.heartbeat = make(chan struct{}, 1)
	client.wg.Add(1)
	go client.sendHeartbeatLoop()
	// handle connection
	// <warning> don't add wg
	go func() {
		defer func() {
			if r := recover(); r != nil {
				client.log(logger.Fatal, xpanic.Print(r, "client.HandleConn"))
			}
			client.Close()
		}()
		protocol.HandleConn(client.conn, client.onFrame)
	}()
	return &client, nil
}

// [2019-12-26 21:44:17] [info] <client> disconnected
// ----------------connected node guid-----------------
// F50B876BE94437E2E678C5EB84627230C599B847BED5B00D5390
// C38C4E155C0DD0305F7A000000005E04B92C00000000000003D5
// -----------------connection status------------------
// local:  tcp 127.0.0.1:2035
// remote: tcp 127.0.0.1:2032
// sent:   5.656 MB received: 5.379 MB
// connect time: 2019-12-26 21:44:13
// ----------------------------------------------------
func (client *client) log(l logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintln(b, log...)
	client.logExtra(l, b)
}

func (client *client) logf(l logger.Level, format string, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, format, log...)
	_, _ = fmt.Fprint(b, "\n")
	client.logExtra(l, b)
}

func (client *client) logExtra(l logger.Level, b *bytes.Buffer) {
	if client.guid != nil {
		const format = "----------------connected node guid-----------------\n%X\n%X\n"
		_, _ = fmt.Fprintf(b, format, client.guid[:guid.Size/2], client.guid[guid.Size/2:])
	}
	const conn = "-----------------connection status------------------\n%s\n"
	_, _ = fmt.Fprintf(b, conn, client.conn)
	const endLine = "----------------------------------------------------"
	_, _ = fmt.Fprint(b, endLine)
	client.ctx.logger.Print(l, "client", b)
}

// Zero—Knowledge Proof
func (client *client) handshake(conn *xnet.Conn) error {
	_ = conn.SetDeadline(client.ctx.global.Now().Add(client.ctx.client.Timeout))
	// about check connection
	sizeByte := make([]byte, 1)
	_, err := io.ReadFull(conn, sizeByte)
	if err != nil {
		return errors.Wrap(err, "failed to receive check connection size")
	}
	size := int(sizeByte[0])
	checkData := make([]byte, size)
	_, err = io.ReadFull(conn, checkData)
	if err != nil {
		return errors.Wrap(err, "failed to receive check connection data")
	}
	_, err = conn.Write(random.New().Bytes(size))
	if err != nil {
		return errors.Wrap(err, "failed to send check connection data")
	}
	// receive certificate
	cert, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive certificate")
	}
	if !client.verifyCertificate(cert, client.node.Address, client.guid) {
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

func (client *client) verifyCertificate(cert []byte, address string, guid []byte) bool {
	// if guid = nil, skip verify
	if guid == nil {
		return true
	}
	if len(cert) != 2*ed25519.SignatureSize {
		return false
	}
	// verify certificate
	buffer := bytes.Buffer{}
	buffer.WriteString(address)
	buffer.Write(guid)
	certWithNodeGUID := cert[:ed25519.SignatureSize]
	return client.ctx.global.Verify(buffer.Bytes(), certWithNodeGUID)
}

func (client *client) isClosed() bool {
	return atomic.LoadInt32(&client.inClose) != 0
}

func (client *client) isSync() bool {
	return atomic.LoadInt32(&client.inSync) != 0
}

// can use client.Close()
func (client *client) onFrame(frame []byte) {
	if client.isClosed() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(frame) < protocol.FrameCMDSize+protocol.FrameIDSize {
		client.log(logger.Exploit, protocol.ErrInvalidFrameSize)
		client.Close()
		return
	}
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	if client.isSync() {
		if client.onFrameAfterSync(frame[0], id, data) {
			return
		}
	}
	switch frame[0] {
	case protocol.ConnReply:
		client.handleReply(frame[protocol.FrameCMDSize:])
	case protocol.ConnReplyHeartbeat:
		select {
		case client.heartbeat <- struct{}{}:
		case <-client.stopSignal:
		}
	case protocol.ConnErrRecvNullFrame:
		client.log(logger.Exploit, protocol.ErrRecvNullFrame)
		client.Close()
	case protocol.ConnErrRecvTooBigFrame:
		client.log(logger.Exploit, protocol.ErrRecvTooBigFrame)
		client.Close()
	case protocol.TestCommand:
		client.reply(id, data)
	default:
		const format = "unknown command: %d\nframe:\n%s"
		client.logf(logger.Exploit, format, frame[0], spew.Sdump(frame))
		client.Close()
	}
}

func (client *client) onFrameAfterSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.NodeSendGUID:
		client.handleNodeSendGUID(id, data)
	case protocol.NodeSend:
		client.handleNodeSend(id, data)
	case protocol.NodeAckGUID:
		client.handleNodeAckGUID(id, data)
	case protocol.NodeAck:
		client.handleNodeAck(id, data)
	case protocol.BeaconSendGUID:
		client.handleBeaconSendGUID(id, data)
	case protocol.BeaconSend:
		client.handleBeaconSend(id, data)
	case protocol.BeaconAckGUID:
		client.handleBeaconAckGUID(id, data)
	case protocol.BeaconAck:
		client.handleBeaconAck(id, data)
	case protocol.BeaconQueryGUID:
		client.handleBeaconQueryGUID(id, data)
	case protocol.BeaconQuery:
		client.handleBeaconQuery(id, data)
	default:
		return false
	}
	return true
}

func (client *client) sendHeartbeatLoop() {
	defer client.wg.Done()
	var err error
	r := random.New()
	buffer := bytes.NewBuffer(nil)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		timer.Reset(time.Duration(30+r.Int(60)) * time.Second)
		select {
		case <-timer.C:
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
			timer.Reset(time.Duration(30+r.Int(60)) * time.Second)
			select {
			case <-client.heartbeat:
			case <-timer.C:
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

func (client *client) reply(id, reply []byte) {
	if client.isClosed() {
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
	_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = client.conn.Write(b)
}

// msg id(2 bytes) + data
func (client *client) handleReply(reply []byte) {
	l := len(reply)
	if l < protocol.FrameIDSize {
		client.log(logger.Exploit, protocol.ErrRecvInvalidFrameIDSize)
		client.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:protocol.FrameIDSize]))
	if id > protocol.MaxFrameID {
		client.log(logger.Exploit, protocol.ErrRecvInvalidFrameID)
		client.Close()
		return
	}
	// must copy
	r := make([]byte, l-protocol.FrameIDSize)
	copy(r, reply[protocol.FrameIDSize:])
	// <security> maybe incorrect msg id
	select {
	case client.slots[id].Reply <- r:
	default:
		client.log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		client.Close()
	}
}

// Sync is used to switch to sync mode
func (client *client) Sync() error {
	client.syncM.Lock()
	defer client.syncM.Unlock()
	if client.isSync() {
		return nil
	}
	resp, err := client.Send(protocol.CtrlSync, nil)
	if err != nil {
		return errors.Wrap(err, "failed to receive sync response")
	}
	if !bytes.Equal(resp, []byte{protocol.NodeSync}) {
		return errors.Errorf("failed to start sync: %s", resp)
	}
	atomic.StoreInt32(&client.inSync, 1)
	return nil
}

func (client *client) handleNodeSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid node send guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckNodeSendGUID(data, false, 0) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid node ack guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckNodeAckGUID(data, false, 0) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid beacon send guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckBeaconSendGUID(data, false, 0) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid beacon ack guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckBeaconAckGUID(data, false, 0) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconQueryGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid query guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckQueryGUID(data, false, 0) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeSend(id, data []byte) {
	s := client.ctx.worker.GetSendFromPool()
	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		client.log(logger.Exploit, "invalid node send msgpack data:", err)
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
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutSendToPool(s)
		return
	}
	if client.ctx.syncer.CheckNodeSendGUID(s.GUID, true, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddNodeSend(s)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(s)
	}
}

func (client *client) handleNodeAck(id, data []byte) {
	a := client.ctx.worker.GetAcknowledgeFromPool()
	err := msgpack.Unmarshal(data, a)
	if err != nil {
		client.log(logger.Exploit, "invalid node ack msgpack data:", err)
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid node ack: %s\n%s", err, spew.Sdump(a))
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutAcknowledgeToPool(a)
		return
	}
	if client.ctx.syncer.CheckNodeAckGUID(a.GUID, true, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddNodeAcknowledge(a)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutAcknowledgeToPool(a)
	}
}

func (client *client) handleBeaconSend(id, data []byte) {
	s := client.ctx.worker.GetSendFromPool()
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		client.log(logger.Exploit, "invalid beacon send msgpack data:", err)
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
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutSendToPool(s)
		return
	}
	if client.ctx.syncer.CheckBeaconSendGUID(s.GUID, true, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddBeaconSend(s)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(s)
	}
}

func (client *client) handleBeaconAck(id, data []byte) {
	a := client.ctx.worker.GetAcknowledgeFromPool()
	err := msgpack.Unmarshal(data, a)
	if err != nil {
		client.log(logger.Exploit, "invalid beacon ack msgpack data:", err)
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon ack: %s\n%s", err, spew.Sdump(a))
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutAcknowledgeToPool(a)
		return
	}
	if client.ctx.syncer.CheckBeaconAckGUID(a.GUID, true, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddBeaconAcknowledge(a)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutAcknowledgeToPool(a)
	}
}

func (client *client) handleBeaconQuery(id, data []byte) {
	q := client.ctx.worker.GetQueryFromPool()
	err := msgpack.Unmarshal(data, q)
	if err != nil {
		client.log(logger.Exploit, "invalid query msgpack data:", err)
		client.ctx.worker.PutQueryToPool(q)
		client.Close()
		return
	}
	err = q.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid query: %s\n%s", err, spew.Sdump(q))
		client.ctx.worker.PutQueryToPool(q)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(q.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutQueryToPool(q)
		return
	}
	if client.ctx.syncer.CheckQueryGUID(q.GUID, true, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddQuery(q)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutQueryToPool(q)
	}
}

// Send is used to send command and receive reply
func (client *client) Send(cmd uint8, data []byte) ([]byte, error) {
	if client.isClosed() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-client.slots[id].Available:
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
					return nil, protocol.ErrRecvReplyTimeout
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

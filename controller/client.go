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

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/rand"
	"project/internal/dns"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
	"project/internal/xpanic"
)

// Client is used to connect Node listener
type Client struct {
	ctx *CTRL

	listener  *bootstrap.Listener
	guid      *guid.GUID // Node GUID
	closeFunc func()

	tag       *guid.GUID
	conn      *xnet.Conn
	rand      *random.Rand
	slots     []*protocol.Slot
	heartbeat chan struct{}
	inSync    int32
	syncM     sync.Mutex

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// NewClient is used to create a client and connect node listener
// when GUID == nil      for trust node
// when GUID != CtrlGUID for sender client
// when GUID == CtrlGUID for discovery
func (ctrl *CTRL) NewClient(
	ctx context.Context,
	bl *bootstrap.Listener,
	guid *guid.GUID,
	closeFunc func(),
) (*Client, error) {
	// dial
	host, port, err := net.SplitHostPort(bl.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	opts := xnet.Options{
		TLSConfig: &tls.Config{
			Rand:       rand.Reader,
			Time:       ctrl.global.Now,
			ServerName: host,
			RootCAs:    x509.NewCertPool(),
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"}, // TODO add config
		},
		Timeout: ctrl.clientMgr.GetTimeout(),
		Now:     ctrl.global.Now,
	}
	// add certificates
	for _, pair := range ctrl.global.GetSystemCerts() {
		opts.TLSConfig.RootCAs.AddCert(pair.Certificate)
	}
	for _, pair := range ctrl.global.GetSelfCerts() {
		opts.TLSConfig.RootCAs.AddCert(pair.Certificate)
	}
	// set proxy
	proxy, err := ctrl.global.GetProxyClient(ctrl.clientMgr.GetProxyTag())
	if err != nil {
		return nil, err
	}
	opts.Dialer = proxy.DialContext
	// resolve domain name
	dnsOpts := ctrl.clientMgr.GetDNSOptions()
	result, err := ctrl.global.ResolveWithContext(ctx, host, dnsOpts)
	if err != nil {
		return nil, err
	}
	var conn *xnet.Conn
	for i := 0; i < len(result); i++ {
		address := net.JoinHostPort(result[i], port)
		conn, err = xnet.DialContext(ctx, bl.Mode, bl.Network, address, &opts)
		if err == nil {
			break
		}
	}
	if conn == nil {
		const format = "failed to connect node listener %s, because %s"
		return nil, errors.Errorf(format, bl, err)
	}

	// handshake
	client := &Client{
		ctx:       ctrl,
		listener:  bl,
		guid:      guid,
		conn:      conn,
		closeFunc: closeFunc,
		rand:      random.New(),
	}
	err = client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		const format = "failed to handshake with node listener: %s"
		return nil, errors.WithMessagef(err, format, bl)
	}

	// initialize message slots
	client.slots = protocol.NewSlots()
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
	ctrl.clientMgr.Add(client)
	client.log(logger.Info, "connected")
	return client, nil
}

// [2019-12-26 21:44:17] [info] <client> disconnected
// --------------connected node guid---------------
// F50B876BE94437E2E678C5EB84627230C599B847BED5B00D
// C38C4E155C0DD0305F7A000000005E04B92C000000000000
// ---------------connection status----------------
// local:  tcp 127.0.0.1:2035
// remote: tcp 127.0.0.1:2032
// sent:   5.656 MB received: 5.379 MB
// mode:   tls,  default network: tcp
// connect time: 2019-12-26 21:44:13
// ------------------------------------------------
func (client *Client) logf(lv logger.Level, format string, log ...interface{}) {
	output := new(bytes.Buffer)
	_, _ = fmt.Fprintf(output, format+"\n", log...)
	client.logExtra(lv, output)
}

func (client *Client) log(lv logger.Level, log ...interface{}) {
	output := new(bytes.Buffer)
	_, _ = fmt.Fprintln(output, log...)
	client.logExtra(lv, output)
}

func (client *Client) logExtra(lv logger.Level, buf *bytes.Buffer) {
	if client.guid != nil {
		const format = "--------------connected node guid---------------\n%s\n"
		_, _ = fmt.Fprintf(buf, format, client.guid.Hex())
	}
	const conn = "---------------connection status----------------\n%s\n"
	_, _ = fmt.Fprintf(buf, conn, client.conn)
	const endLine = "------------------------------------------------"
	_, _ = fmt.Fprint(buf, endLine)
	client.ctx.logger.Print(lv, "client", buf)
}

// Zero—Knowledge Proof
func (client *Client) handshake(conn *xnet.Conn) error {
	timeout := client.ctx.clientMgr.GetTimeout()
	_ = conn.SetDeadline(client.ctx.global.Now().Add(timeout))
	// check connection
	err := client.checkConn(conn)
	if err != nil {
		return err
	}
	// verify certificate
	publicKey := client.ctx.global.PublicKey()
	ok, err := protocol.VerifyCertificate(conn, publicKey, client.guid)
	if err != nil {
		client.log(logger.Exploit, err)
		return err
	}
	if !ok {
		return errors.New("failed to verify node certificate")
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
	// maybe fake node will send some special data
	// and if controller sign it will destroy net
	if len(challenge) != protocol.ChallengeSize {
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
	if bytes.Compare(resp, protocol.AuthSucceed) != 0 {
		err = errors.WithStack(protocol.ErrAuthenticateFailed)
		client.log(logger.Exploit, err)
		return err
	}
	return conn.SetDeadline(time.Time{})
}

// TODO improve it, server side use "white" method
func (client *Client) checkConn(conn *xnet.Conn) error {
	size := byte(100 + client.rand.Int(156))
	data := client.rand.Bytes(int(size))
	_, err := conn.Write(append([]byte{size}, data...))
	if err != nil {
		return errors.WithMessage(err, "failed to send check connection data")
	}
	n, err := io.ReadFull(conn, data)
	if err != nil {
		const format = "error in client.checkConn(): %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(data[:n]))
		return err
	}
	return nil
}

func (client *Client) sendHeartbeatLoop() {
	defer client.wg.Done()
	var err error
	buffer := bytes.NewBuffer(nil)
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		timer.Reset(time.Duration(30+client.rand.Int(60)) * time.Second)
		select {
		case <-timer.C:
			// <security> fake traffic like client
			fakeSize := 64 + client.rand.Int(256)
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			buffer.Reset()
			buffer.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
			buffer.WriteByte(protocol.ConnSendHeartbeat)
			buffer.Write(client.rand.Bytes(fakeSize))
			// send
			_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err = client.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			// receive reply
			timer.Reset(time.Duration(30+client.rand.Int(60)) * time.Second)
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

func (client *Client) isClosed() bool {
	return atomic.LoadInt32(&client.inClose) != 0
}

func (client *Client) isSync() bool {
	return atomic.LoadInt32(&client.inSync) != 0
}

// can use client.Close()
func (client *Client) onFrame(frame []byte) {
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

func (client *Client) reply(id, reply []byte) {
	if client.isClosed() {
		return
	}
	l := len(reply)
	// 7 = size(4 Bytes) + ConnReply(1 byte) + msg id(2 bytes)
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
func (client *Client) handleReply(reply []byte) {
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

// Synchronize is used to switch to synchronize mode
func (client *Client) Synchronize() error {
	client.syncM.Lock()
	defer client.syncM.Unlock()
	if client.isSync() {
		return nil
	}
	resp, err := client.send(protocol.CtrlSync, nil)
	if err != nil {
		return errors.Wrap(err, "failed to receive synchronize response")
	}
	if bytes.Compare(resp, []byte{protocol.NodeSync}) != 0 {
		return errors.Errorf("failed to start synchronize: %s", resp)
	}
	atomic.StoreInt32(&client.inSync, 1)
	const format = "start synchronize\nlistener: %s"
	client.logf(logger.Info, format, client.listener)
	return nil
}

func (client *Client) onFrameAfterSync(cmd byte, id, data []byte) bool {
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

func (client *Client) handleNodeSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid node send guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckNodeSendGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleNodeAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid node ack guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckNodeAckGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleBeaconSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid beacon send guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckBeaconSendGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleBeaconAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid beacon ack guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckBeaconAckGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleBeaconQueryGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid query guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckQueryGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleNodeSend(id, data []byte) {
	send := client.ctx.worker.GetSendFromPool()
	err := send.Unpack(data)
	if err != nil {
		const format = "invalid node send data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(send))
		client.ctx.worker.PutSendToPool(send)
		client.Close()
		return
	}
	err = send.Validate()
	if err != nil {
		const format = "invalid node send: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(send))
		client.ctx.worker.PutSendToPool(send)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(&send.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutSendToPool(send)
		return
	}
	if client.ctx.syncer.CheckNodeSendGUID(&send.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddNodeSend(send)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(send)
	}
}

func (client *Client) handleNodeAck(id, data []byte) {
	ack := client.ctx.worker.GetAcknowledgeFromPool()
	err := ack.Unpack(data)
	if err != nil {
		const format = "invalid node ack data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(ack))
		client.ctx.worker.PutAcknowledgeToPool(ack)
		client.Close()
		return
	}
	err = ack.Validate()
	if err != nil {
		const format = "invalid node ack: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(ack))
		client.ctx.worker.PutAcknowledgeToPool(ack)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(&ack.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutAcknowledgeToPool(ack)
		return
	}
	if client.ctx.syncer.CheckNodeAckGUID(&ack.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddNodeAcknowledge(ack)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutAcknowledgeToPool(ack)
	}
}

func (client *Client) handleBeaconSend(id, data []byte) {
	s := client.ctx.worker.GetSendFromPool()
	err := s.Unpack(data)
	if err != nil {
		const format = "invalid beacon send data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(s))
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid beacon send: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(s))
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(&s.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutSendToPool(s)
		return
	}
	if client.ctx.syncer.CheckBeaconSendGUID(&s.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddBeaconSend(s)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(s)
	}
}

func (client *Client) handleBeaconAck(id, data []byte) {
	a := client.ctx.worker.GetAcknowledgeFromPool()
	err := a.Unpack(data)
	if err != nil {
		const format = "invalid beacon ack data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(a))
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid beacon ack: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(a))
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(&a.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutAcknowledgeToPool(a)
		return
	}
	if client.ctx.syncer.CheckBeaconAckGUID(&a.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddBeaconAcknowledge(a)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutAcknowledgeToPool(a)
	}
}

func (client *Client) handleBeaconQuery(id, data []byte) {
	q := client.ctx.worker.GetQueryFromPool()
	err := q.Unpack(data)
	if err != nil {
		const format = "invalid query data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(q))
		client.ctx.worker.PutQueryToPool(q)
		client.Close()
		return
	}
	err = q.Validate()
	if err != nil {
		const format = "invalid query: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(q))
		client.ctx.worker.PutQueryToPool(q)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(&q.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutQueryToPool(q)
		return
	}
	if client.ctx.syncer.CheckQueryGUID(&q.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddQuery(q)
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutQueryToPool(q)
	}
}

// send is used to send command and receive reply
func (client *Client) send(cmd uint8, data []byte) ([]byte, error) {
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
				client.slots[id].Timer.Reset(protocol.RecvTimeout)
				select {
				case r := <-client.slots[id].Reply:
					client.slots[id].Timer.Stop()
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

// SendCommand is used to send command and receive reply
func (client *Client) SendCommand(cmd uint8, data []byte) ([]byte, error) {
	return client.send(cmd, data)
}

// SendToNode is used to send message to node
func (client *Client) SendToNode(guid *guid.GUID, data *bytes.Buffer) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, sr.Err = client.send(protocol.CtrlSendToNodeGUID, guid[:])
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = client.send(protocol.CtrlSendToNode, data.Bytes())
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		sr.Err = protocol.GetReplyError(reply)
	}
	return
}

// SendToBeacon is used to send message to beacon.
func (client *Client) SendToBeacon(guid *guid.GUID, data *bytes.Buffer) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, sr.Err = client.send(protocol.CtrlSendToBeaconGUID, guid[:])
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = client.send(protocol.CtrlSendToBeacon, data.Bytes())
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		sr.Err = protocol.GetReplyError(reply)
	}
	return
}

// AcknowledgeToNode is used to notice Node that Controller has received this message.
func (client *Client) AcknowledgeToNode(
	guid *guid.GUID,
	data *bytes.Buffer,
) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.send(protocol.CtrlAckToNodeGUID, guid[:])
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	reply, ar.Err = client.send(protocol.CtrlAckToNode, data.Bytes())
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		ar.Err = errors.New(string(reply))
	}
	return
}

// AcknowledgeToBeacon is used to notice Beacon that Controller has received this message.
func (client *Client) AcknowledgeToBeacon(
	guid *guid.GUID,
	data *bytes.Buffer,
) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.send(protocol.CtrlAckToBeaconGUID, guid[:])
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	reply, ar.Err = client.send(protocol.CtrlAckToBeacon, data.Bytes())
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		ar.Err = errors.New(string(reply))
	}
	return
}

// Broadcast is used to broadcast message to all Nodes.
func (client *Client) Broadcast(guid *guid.GUID, data *bytes.Buffer) (br *protocol.BroadcastResponse) {
	br = &protocol.BroadcastResponse{
		GUID: client.guid,
	}
	var reply []byte
	reply, br.Err = client.send(protocol.CtrlBroadcastGUID, guid[:])
	if br.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		br.Err = protocol.GetReplyError(reply)
		return
	}
	// broadcast
	reply, br.Err = client.send(protocol.CtrlBroadcast, data.Bytes())
	if br.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		br.Err = protocol.GetReplyError(reply)
	}
	return
}

// Answer is used to return the result of the Beacon query.
func (client *Client) Answer(guid *guid.GUID, data *bytes.Buffer) (ar *protocol.AnswerResponse) {
	ar = &protocol.AnswerResponse{
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.send(protocol.CtrlAnswerGUID, guid[:])
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		return
	}
	reply, ar.Err = client.send(protocol.CtrlAnswer, data.Bytes())
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		ar.Err = errors.New(string(reply))
	}
	return
}

// Status is used to get connection status
func (client *Client) Status() *xnet.Status {
	return client.conn.Status()
}

// Close is used to disconnect Node
func (client *Client) Close() {
	client.closeOnce.Do(func() {
		atomic.StoreInt32(&client.inClose, 1)
		_ = client.conn.Close()
		close(client.stopSignal)
		protocol.DestroySlots(client.slots)
		client.wg.Wait()
		client.ctx.clientMgr.Delete(client.tag)
		if client.closeFunc != nil {
			client.closeFunc()
		}
		client.log(logger.Info, "disconnected")
	})
}

// clientMgr contains all clients from NewClient() and client options from Config
// it can generate client tag, you can manage all clients here
type clientMgr struct {
	ctx *CTRL

	// options from Config
	proxyTag string
	timeout  time.Duration
	dnsOpts  dns.Options
	optsRWM  sync.RWMutex

	guid *guid.Generator

	clients    map[guid.GUID]*Client
	clientsRWM sync.RWMutex
}

func newClientManager(ctx *CTRL, config *Config) (*clientMgr, error) {
	cfg := config.Client

	if cfg.Timeout < 10*time.Second {
		return nil, errors.New("client timeout must >= 10 seconds")
	}

	return &clientMgr{
		ctx:      ctx,
		proxyTag: cfg.ProxyTag,
		timeout:  cfg.Timeout,
		dnsOpts:  cfg.DNSOpts,
		guid:     guid.New(4, ctx.global.Now),
		clients:  make(map[guid.GUID]*Client),
	}, nil
}

func (cm *clientMgr) GetProxyTag() string {
	cm.optsRWM.RLock()
	defer cm.optsRWM.RUnlock()
	return cm.proxyTag
}

func (cm *clientMgr) GetTimeout() time.Duration {
	cm.optsRWM.RLock()
	defer cm.optsRWM.RUnlock()
	return cm.timeout
}

func (cm *clientMgr) GetDNSOptions() *dns.Options {
	cm.optsRWM.RLock()
	defer cm.optsRWM.RUnlock()
	return cm.dnsOpts.Clone()
}

func (cm *clientMgr) SetProxyTag(tag string) error {
	// check proxy is exist
	_, err := cm.ctx.global.GetProxyClient(tag)
	if err != nil {
		return err
	}
	cm.optsRWM.Lock()
	defer cm.optsRWM.Unlock()
	cm.proxyTag = tag
	return nil
}

func (cm *clientMgr) SetTimeout(timeout time.Duration) error {
	if timeout < 10*time.Second {
		return errors.New("timeout must >= 10 seconds")
	}
	cm.optsRWM.Lock()
	defer cm.optsRWM.Unlock()
	cm.timeout = timeout
	return nil
}

func (cm *clientMgr) SetDNSOptions(opts *dns.Options) {
	cm.optsRWM.Lock()
	defer cm.optsRWM.Unlock()
	cm.dnsOpts = *opts.Clone()
}

// for NewClient()
func (cm *clientMgr) Add(client *Client) {
	client.tag = cm.guid.Get()
	cm.clientsRWM.Lock()
	defer cm.clientsRWM.Unlock()
	if _, ok := cm.clients[*client.tag]; !ok {
		cm.clients[*client.tag] = client
	}
}

// for client.Close()
func (cm *clientMgr) Delete(tag *guid.GUID) {
	cm.clientsRWM.Lock()
	defer cm.clientsRWM.Unlock()
	delete(cm.clients, *tag)
}

// Clients is used to get all clients
func (cm *clientMgr) Clients() map[guid.GUID]*Client {
	cm.clientsRWM.RLock()
	defer cm.clientsRWM.RUnlock()
	clients := make(map[guid.GUID]*Client, len(cm.clients))
	for tag, client := range cm.clients {
		clients[tag] = client
	}
	return clients
}

// Kill is used to close client. Must use cm.Clients(),
// because client.Close() will use cm.clientsRWM
func (cm *clientMgr) Kill(tag *guid.GUID) {
	if client, ok := cm.Clients()[*tag]; ok {
		client.Close()
	}
}

// Close will close all active clients
func (cm *clientMgr) Close() {
	for {
		for _, client := range cm.Clients() {
			client.Close()
		}
		time.Sleep(10 * time.Millisecond)
		if len(cm.Clients()) == 0 {
			break
		}
	}
	cm.guid.Close()
	cm.ctx = nil
}

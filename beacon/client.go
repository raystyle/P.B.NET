package beacon

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

// Client is used to connect Node's listener.
type Client struct {
	ctx *Beacon

	listener  *bootstrap.Listener
	guid      *guid.GUID // Node GUID
	closeFunc func()

	tag       *guid.GUID
	Conn      *xnet.Conn
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
// when guid != ctrl guid for forwarder
// when guid == ctrl guid for register
func (beacon *Beacon) NewClient(
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
			Time:       beacon.global.Now,
			ServerName: host,
			RootCAs:    x509.NewCertPool(),
			MinVersion: tls.VersionTLS12,
		},
		Timeout: beacon.clientMgr.GetTimeout(),
		Now:     beacon.global.Now,
	}
	// add CA certificates
	for _, cert := range beacon.global.Certificates() {
		opts.TLSConfig.RootCAs.AddCert(cert)
	}
	// set proxy
	proxy, err := beacon.global.GetProxyClient(beacon.clientMgr.GetProxyTag())
	if err != nil {
		return nil, err
	}
	opts.Dialer = proxy.DialContext
	// resolve domain name
	dnsOpts := beacon.clientMgr.GetDNSOptions()
	result, err := beacon.global.ResolveWithContext(ctx, host, dnsOpts)
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
		return nil, errors.Errorf("failed to connect node listener: %s", bl)
	}

	// handshake
	client := &Client{
		ctx:       beacon,
		listener:  bl,
		guid:      guid,
		Conn:      conn,
		closeFunc: closeFunc,
		rand:      random.New(),
	}
	err = client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		const format = "failed to handshake with node listener: %s"
		return nil, errors.WithMessagef(err, format, bl)
	}
	beacon.clientMgr.Add(client)
	client.log(logger.Info, "create client")
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
	if *client.guid != *protocol.CtrlGUID {
		const format = "--------------connected node guid---------------\n%s\n"
		_, _ = fmt.Fprintf(buf, format, client.guid.Hex())
	}
	const conn = "---------------connection status----------------\n%s\n"
	_, _ = fmt.Fprintf(buf, conn, client.Conn)
	const endLine = "------------------------------------------------"
	_, _ = fmt.Fprint(buf, endLine)
	client.ctx.logger.Print(lv, "client", buf)
}

func (client *Client) handshake(conn *xnet.Conn) error {
	timeout := client.ctx.clientMgr.GetTimeout()
	_ = conn.SetDeadline(client.ctx.global.Now().Add(timeout))
	// about check connection
	err := client.checkConn(conn)
	if err != nil {
		return err
	}
	// verify certificate
	publicKey := client.ctx.global.CtrlPublicKey()
	ok, err := protocol.VerifyCertificate(conn, publicKey, client.guid)
	if err != nil {
		client.log(logger.Exploit, err)
		return err
	}
	if !ok {
		return errors.New("failed to verify node certificate")
	}
	// send role
	_, err = conn.Write(protocol.Beacon.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send role")
	}
	// send self guid
	_, err = conn.Write(client.ctx.global.GUID()[:])
	return err
}

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

// Connect is used to start protocol.HandleConn(), if you want to
// start Synchronize(), you must call this function first.
func (client *Client) Connect() error {
	// send connect operation
	_, err := client.Conn.Write([]byte{protocol.BeaconOperationConnect})
	if err != nil {
		return errors.Wrap(err, "failed to send connect operation")
	}
	err = client.authenticate()
	if err != nil {
		return err
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
		protocol.HandleConn(client.Conn, client.onFrame)
	}()
	timeout := client.ctx.clientMgr.GetTimeout()
	_ = client.Conn.SetDeadline(client.ctx.global.Now().Add(timeout))
	client.log(logger.Debug, "connected")
	return nil
}

func (client *Client) authenticate() error {
	// receive challenge
	challenge, err := client.Conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive challenge")
	}
	if len(challenge) != protocol.ChallengeSize {
		err = errors.New("invalid challenge size")
		client.log(logger.Exploit, err)
		return err
	}
	// send signature
	err = client.Conn.Send(client.ctx.global.Sign(challenge))
	if err != nil {
		return errors.Wrap(err, "failed to send challenge signature")
	}
	resp, err := client.Conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive authentication response")
	}
	if bytes.Compare(resp, protocol.AuthSucceed) != 0 {
		return errors.WithStack(protocol.ErrAuthenticateFailed)
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
			_ = client.Conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err = client.Conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			// receive reply
			timer.Reset(time.Duration(30+client.rand.Int(60)) * time.Second)
			select {
			case <-client.heartbeat:
			case <-timer.C:
				client.log(logger.Warning, "receive heartbeat timeout")
				_ = client.Conn.Close()
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
	_ = client.Conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = client.Conn.Write(b)
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
		return errors.New("already synchronize")
	}
	// must presume, or may be lost message.
	var err error
	atomic.StoreInt32(&client.inSync, 1)
	defer func() {
		if err != nil {
			atomic.StoreInt32(&client.inSync, 0)
		}
	}()
	resp, err := client.send(protocol.BeaconSync, nil)
	if err != nil {
		return errors.Wrap(err, "failed to receive synchronize response")
	}
	if bytes.Compare(resp, []byte{protocol.NodeSync}) != 0 {
		err = errors.Errorf("failed to start to synchronize: %s", resp)
		return err // can't return directly
	}
	client.logf(logger.Info, "start to synchronize\nlistener: %s", client.listener)
	return nil
}

func (client *Client) onFrameAfterSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSendToBeaconGUID:
		client.handleSendToBeaconGUID(id, data)
	case protocol.CtrlSendToBeacon:
		client.handleSendToBeacon(id, data)
	case protocol.CtrlAckToBeaconGUID:
		client.handleAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		client.handleAckToBeacon(id, data)
	case protocol.CtrlAnswerGUID:
		client.handleAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		client.handleAnswer(id, data)
	default:
		return false
	}
	return true
}

func (client *Client) handleSendToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid send to beacon guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckSendToBeaconGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleAckToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ack to beacon guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckAckToBeaconGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleAnswerGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid answer guid size")
		client.reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if client.ctx.syncer.CheckGUIDSliceTimestamp(data) {
		client.reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckAnswerGUIDSlice(data) {
		client.reply(id, protocol.ReplyUnhandled)
	} else {
		client.reply(id, protocol.ReplyHandled)
	}
}

func (client *Client) handleSendToBeacon(id, data []byte) {
	send := client.ctx.worker.GetSendFromPool()
	err := send.Unpack(data)
	if err != nil {
		const format = "invalid send to beacon data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(send))
		client.ctx.worker.PutSendToPool(send)
		client.Close()
		return
	}
	err = send.Validate()
	if err != nil {
		const format = "invalid send to beacon: %s\n%s"
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
	if client.ctx.syncer.CheckSendToBeaconGUID(&send.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		if send.RoleGUID == *client.ctx.global.GUID() {
			client.ctx.worker.AddSend(send)
		} else {
			const format = "invalid beacon guid in send to beacon\n%s"
			client.logf(logger.Exploit, format, send)
			client.ctx.worker.PutSendToPool(send)
		}
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(send)
	}
}

func (client *Client) handleAckToBeacon(id, data []byte) {
	ack := client.ctx.worker.GetAcknowledgeFromPool()
	err := ack.Unpack(data)
	if err != nil {
		const format = "invalid ack to beacon data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(ack))
		client.ctx.worker.PutAcknowledgeToPool(ack)
		client.Close()
		return
	}
	err = ack.Validate()
	if err != nil {
		const format = "invalid ack to beacon: %s\n%s"
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
	if client.ctx.syncer.CheckAckToBeaconGUID(&ack.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		if ack.RoleGUID == *client.ctx.global.GUID() {
			client.ctx.worker.AddAcknowledge(ack)
		} else {
			const format = "invalid beacon guid in ack to beacon\n%s"
			client.logf(logger.Exploit, format, ack)
			client.ctx.worker.PutAcknowledgeToPool(ack)
		}
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutAcknowledgeToPool(ack)
	}
}

func (client *Client) handleAnswer(id, data []byte) {
	answer := client.ctx.worker.GetAnswerFromPool()
	err := answer.Unpack(data)
	if err != nil {
		const format = "invalid answer data: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(answer))
		client.ctx.worker.PutAnswerToPool(answer)
		client.Close()
		return
	}
	err = answer.Validate()
	if err != nil {
		const format = "invalid answer: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(answer))
		client.ctx.worker.PutAnswerToPool(answer)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(&answer.GUID)
	if expired {
		client.reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutAnswerToPool(answer)
		return
	}
	if client.ctx.syncer.CheckAnswerGUID(&answer.GUID, timestamp) {
		client.reply(id, protocol.ReplySucceed)
		if answer.BeaconGUID == *client.ctx.global.GUID() {
			client.ctx.worker.AddAnswer(answer)
		} else {
			const format = "invalid beacon guid in answer\n%s"
			client.logf(logger.Exploit, format, answer)
			client.ctx.worker.PutAnswerToPool(answer)
		}
	} else {
		client.reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutAnswerToPool(answer)
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
				_ = client.Conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := client.Conn.Write(b)
				if err != nil {
					_ = client.Conn.Close()
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

// Send is used to send message to Controller.
func (client *Client) Send(
	guid *guid.GUID,
	data *bytes.Buffer,
) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, sr.Err = client.send(protocol.BeaconSendGUID, guid[:])
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = client.send(protocol.BeaconSend, data.Bytes())
	if sr.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		sr.Err = errors.New(string(reply))
	}
	return
}

// Acknowledge is used to notice Controller that Beacon has received this message.
func (client *Client) Acknowledge(
	guid *guid.GUID,
	data *bytes.Buffer,
) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.send(protocol.BeaconAckGUID, guid[:])
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		ar.Err = protocol.GetReplyError(reply)
		return
	}
	reply, ar.Err = client.send(protocol.BeaconAck, data.Bytes())
	if ar.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		ar.Err = errors.New(string(reply))
	}
	return
}

// Query is used to query message from Controller.
func (client *Client) Query(
	guid *guid.GUID,
	data *bytes.Buffer,
) (q *protocol.QueryResponse) {
	q = &protocol.QueryResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, q.Err = client.send(protocol.BeaconQueryGUID, guid[:])
	if q.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplyUnhandled) != 0 {
		q.Err = protocol.GetReplyError(reply)
		return
	}
	reply, q.Err = client.send(protocol.BeaconQuery, data.Bytes())
	if q.Err != nil {
		return
	}
	if bytes.Compare(reply, protocol.ReplySucceed) != 0 {
		q.Err = errors.New(string(reply))
	}
	return
}

// Status is used to get connection status
func (client *Client) Status() *xnet.Status {
	return client.Conn.Status()
}

// Close is used to disconnect Node
func (client *Client) Close() {
	client.closeOnce.Do(func() {
		atomic.StoreInt32(&client.inClose, 1)
		_ = client.Conn.Close()
		if client.stopSignal != nil {
			close(client.stopSignal)
			protocol.DestroySlots(client.slots)
		}
		client.wg.Wait()
		client.ctx.clientMgr.Delete(client.tag)
		if client.closeFunc != nil {
			client.closeFunc()
		}
		if client.stopSignal != nil {
			client.log(logger.Info, "disconnected")
		}
	})
}

// clientMgr contains all clients from NewClient() and client options from Config
// it can generate client tag, you can manage all clients here
type clientMgr struct {
	ctx *Beacon

	// options from Config
	proxyTag string
	timeout  time.Duration
	dnsOpts  dns.Options
	optsRWM  sync.RWMutex

	guid       *guid.Generator
	clients    map[guid.GUID]*Client
	clientsRWM sync.RWMutex
}

func newClientManager(ctx *Beacon, config *Config) (*clientMgr, error) {
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

// Clients is used to get all clients.
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
// because client.Close() will use cm.clientsRWM.
func (cm *clientMgr) Kill(tag *guid.GUID) {
	if client, ok := cm.Clients()[*tag]; ok {
		client.Close()
	}
}

// Close will close all active clients.
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

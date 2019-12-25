package node

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	"project/internal/crypto/aes"
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
	ctx *Node

	node      *bootstrap.Node
	guid      []byte // node guid
	closeFunc func()

	conn      *conn
	heartbeat chan struct{}
	inSync    int32

	sendPool   sync.Pool
	ackPool    sync.Pool
	answerPool sync.Pool
	queryPool  sync.Pool

	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// when guid != ctrl guid for forwarder
// when guid == ctrl guid for register
// switch Register() or Connect() after newClient()
func newClient(
	ctx context.Context,
	node *Node,
	n *bootstrap.Node,
	guid []byte,
	closeFunc func(),
) (*client, error) {
	host, port, err := net.SplitHostPort(n.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cfg := xnet.Config{
		Network: n.Network,
		Timeout: node.client.Timeout,
	}
	cfg.TLSConfig = &tls.Config{
		Rand:       rand.Reader,
		Time:       node.global.Now,
		ServerName: host,
		RootCAs:    x509.NewCertPool(),
		MinVersion: tls.VersionTLS12,
	}
	// add CA certificates
	for _, cert := range node.global.Certificates() {
		cfg.TLSConfig.RootCAs.AddCert(cert)
	}
	// set proxy
	p, _ := node.global.GetProxyClient(node.client.ProxyTag)
	cfg.Dialer = p.DialContext
	// resolve domain name
	result, err := node.global.ResolveWithContext(ctx, host, &node.client.DNSOpts)
	if err != nil {
		return nil, err
	}
	var conn *xnet.Conn
	for i := 0; i < len(result); i++ {
		cfg.Address = net.JoinHostPort(result[i], port)
		c, err := xnet.DialContext(ctx, n.Mode, &cfg)
		if err == nil {
			conn = xnet.NewConn(c, node.global.Now())
			break
		}
	}
	if conn == nil {
		return nil, errors.Errorf("failed to connect node: %s", n.Address)
	}

	// handshake
	client := client{
		ctx:       node,
		node:      n,
		guid:      guid,
		closeFunc: closeFunc,
	}
	err = client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		const format = "failed to handshake with node: %s"
		return nil, errors.WithMessagef(err, format, n.Address)
	}
	client.conn = newConn(node.logger, conn, connUsageClient)
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
	_, err = conn.Write(protocol.Node.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send role")
	}
	// send self guid
	_, err = conn.Write(client.ctx.global.GUID())
	return err
}

func (client *client) verifyCertificate(cert []byte, address string, guid []byte) bool {
	if len(cert) != 2*ed25519.SignatureSize {
		return false
	}
	// verify certificate
	buffer := bytes.Buffer{}
	buffer.WriteString(address)
	buffer.Write(guid)
	if bytes.Equal(guid, protocol.CtrlGUID) {
		certWithCtrlGUID := cert[ed25519.SignatureSize:]
		return client.ctx.global.CtrlVerify(buffer.Bytes(), certWithCtrlGUID)
	}
	certWithNodeGUID := cert[:ed25519.SignatureSize]
	return client.ctx.global.CtrlVerify(buffer.Bytes(), certWithNodeGUID)
}

// if return error, must close manually
func (client *client) Connect() error {
	// send operation
	_, err := client.conn.Write([]byte{2})
	if err != nil {
		return errors.Wrap(err, "failed to send operation")
	}
	err = client.authenticate()
	if err != nil {
		return err
	}
	client.heartbeat = make(chan struct{}, 1)
	client.stopSignal = make(chan struct{})
	// <warning> not add wg
	go func() {
		defer func() {
			if r := recover(); r != nil {
				client.log(logger.Fatal, xpanic.Error(r, "client.HandleConn"))
			}
			client.Close()
		}()
		protocol.HandleConn(client.conn, client.onFrame)
	}()
	client.wg.Add(1)
	go client.sendHeartbeatLoop()
	return nil
}

func (client *client) authenticate() error {
	// receive challenge
	challenge, err := client.conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive challenge")
	}
	if len(challenge) < 2048 || len(challenge) > 4096 {
		err = errors.New("invalid challenge size")
		client.log(logger.Exploit, err)
		return err
	}
	// send signature
	err = client.conn.SendRaw(client.ctx.global.Sign(challenge))
	if err != nil {
		return errors.Wrap(err, "failed to send challenge signature")
	}
	resp, err := client.conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive authentication response")
	}
	if !bytes.Equal(resp, protocol.AuthSucceed) {
		return errors.WithStack(protocol.ErrAuthenticateFailed)
	}
	return nil
}

func (client *client) isSync() bool {
	return atomic.LoadInt32(&client.inSync) != 0
}

func (client *client) onFrame(frame []byte) {
	if client.conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnReplyHeartbeat {
		select {
		case client.heartbeat <- struct{}{}:
		case <-client.stopSignal:
		}
	}
	id := frame[protocol.MsgCMDSize : protocol.MsgCMDSize+protocol.MsgIDSize]
	data := frame[protocol.MsgCMDSize+protocol.MsgIDSize:]
	if client.isSync() {
		if client.onFrameAfterSync(frame[0], id, data) {
			return
		}
	}
	client.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
	client.Close()
}

func (client *client) onFrameAfterSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSendToNodeGUID:
		client.handleSendToNodeGUID(id, data)
	case protocol.CtrlSendToNode:
		client.handleSendToNode(id, data)
	case protocol.CtrlAckToNodeGUID:
		client.handleAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		client.handleAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		client.handleSendToBeaconGUID(id, data)
	case protocol.CtrlSendToBeacon:
		client.handleSendToBeacon(id, data)
	case protocol.CtrlAckToBeaconGUID:
		client.handleAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		client.handleAckToBeacon(id, data)
	case protocol.CtrlBroadcastGUID:
		client.handleBroadcastGUID(id, data)
	case protocol.CtrlBroadcast:
		client.handleBroadcast(id, data)
	case protocol.CtrlAnswerGUID:
		client.handleAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		client.handleAnswer(id, data)
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

// Sync is used to switch to sync mode
func (client *client) Sync() error {
	resp, err := client.conn.Send(protocol.NodeSync, nil)
	if err != nil {
		return errors.Wrap(err, "failed to receive sync response")
	}
	if !bytes.Equal(resp, []byte{protocol.NodeSync}) {
		return errors.Errorf("failed to start sync: %s", resp)
	}

	// init sync pool
	client.sendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	client.ackPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	client.answerPool.New = func() interface{} {
		return &protocol.Answer{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Message:    make([]byte, aes.BlockSize),
			Hash:       make([]byte, sha256.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	client.queryPool.New = func() interface{} {
		return &protocol.Query{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	atomic.StoreInt32(&client.inSync, 1)
	return nil
}

func (client *client) handleSendToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl send to node guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckSendToNodeGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleSendToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl send to beacon guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckSendToBeaconGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleAckToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl ack to node guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckAckToNodeGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleAckToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl ack to beacon guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckAckToBeaconGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBroadcastGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl broadcast guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckBroadcastGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleAnswerGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl answer guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckAnswerGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid node send guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckNodeSendGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid node ack guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckNodeAckGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconSendGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid beacon send guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckBeaconSendGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconAckGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid beacon ack guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckBeaconAckGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconQueryGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid beacon query guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckQueryGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleSendToNode(id, data []byte) {
	s := client.ctx.worker.GetSendFromPool()
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		const format = "invalid send to node msgpack data: %s"
		client.logf(logger.Exploit, format, err)
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid send to node: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(s))
		client.ctx.worker.PutSendToPool(s)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutSendToPool(s)
		return
	}
	if client.ctx.syncer.CheckSendToNodeGUID(s.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
		if bytes.Equal(s.RoleGUID, client.ctx.global.GUID()) {
			client.ctx.worker.AddSend(s)
		} else {
			// repeat
			client.ctx.worker.PutSendToPool(s)
		}
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutSendToPool(s)
	}
}

func (client *client) handleAckToNode(id, data []byte) {
	a := client.ctx.worker.GetAcknowledgeFromPool()

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid ack to node msgpack data: %s"
		client.logf(logger.Exploit, format, err)
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid ack to node: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(a))
		client.ctx.worker.PutAcknowledgeToPool(a)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutAcknowledgeToPool(a)
		return
	}
	if client.ctx.syncer.CheckAckToNodeGUID(a.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
		if bytes.Equal(a.RoleGUID, client.ctx.global.GUID()) {
			client.ctx.worker.AddAcknowledge(a)

		} else {
			// repeat
			client.ctx.worker.PutAcknowledgeToPool(a)
		}
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutAcknowledgeToPool(a)
	}
}

func (client *client) handleSendToBeacon(id, data []byte) {
	s := client.sendPool.Get().(*protocol.Send)
	defer client.sendPool.Put(s)
	err := msgpack.Unmarshal(data, s)
	if err != nil {
		const format = "invalid send to beacon msgpack data: %s"
		client.logf(logger.Exploit, format, err)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		const format = "invalid send to beacon: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(s))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckSendToBeaconGUID(s.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleAckToBeacon(id, data []byte) {
	a := client.ackPool.Get().(*protocol.Acknowledge)
	defer client.ackPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid ack to beacon msgpack data: %s"
		client.logf(logger.Exploit, format, err)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid ack to beacon: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(a))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckAckToBeaconGUID(a.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBroadcast(id, data []byte) {
	b := client.ctx.worker.GetBroadcastFromPool()
	err := msgpack.Unmarshal(data, b)
	if err != nil {
		const format = "invalid broadcast msgpack data: %s"
		client.logf(logger.Exploit, format, err)
		client.ctx.worker.PutBroadcastToPool(b)
		client.Close()
		return
	}
	err = b.Validate()
	if err != nil {
		const format = "invalid broadcast: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(b))
		client.ctx.worker.PutBroadcastToPool(b)
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(b.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		client.ctx.worker.PutBroadcastToPool(b)
		return
	}
	if client.ctx.syncer.CheckBroadcastGUID(b.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
		client.ctx.worker.AddBroadcast(b)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
		client.ctx.worker.PutBroadcastToPool(b)
	}
}

func (client *client) handleAnswer(id, data []byte) {
	a := client.answerPool.Get().(*protocol.Answer)
	defer client.answerPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		const format = "invalid answer msgpack data: %s"
		client.logf(logger.Exploit, format, err)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		const format = "invalid answer: %s\n%s"
		client.logf(logger.Exploit, format, err, spew.Sdump(a))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckAnswerGUID(a.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
		// repeat
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeSend(id, data []byte) {
	s := client.sendPool.Get().(*protocol.Send)
	defer client.sendPool.Put(s)

	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		client.log(logger.Exploit, "invalid node send msgpack data:", err)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid node send: %s\n%s", err, spew.Sdump(s))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckNodeSendGUID(s.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleNodeAck(id, data []byte) {
	a := client.ackPool.Get().(*protocol.Acknowledge)
	defer client.ackPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		client.log(logger.Exploit, "invalid node ack msgpack data:", err)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid node ack: %s\n%s", err, spew.Sdump(a))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckNodeAckGUID(a.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconSend(id, data []byte) {
	s := client.sendPool.Get().(*protocol.Send)
	defer client.sendPool.Put(s)

	err := msgpack.Unmarshal(data, s)
	if err != nil {
		client.log(logger.Exploit, "invalid beacon send msgpack data:", err)
		client.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon send: %s\n%s", err, spew.Sdump(s))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(s.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckBeaconSendGUID(s.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconAck(id, data []byte) {
	a := client.ackPool.Get().(*protocol.Acknowledge)
	defer client.ackPool.Put(a)

	err := msgpack.Unmarshal(data, a)
	if err != nil {
		client.log(logger.Exploit, "invalid beacon ack msgpack data:", err)
		client.Close()
		return
	}
	err = a.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon ack: %s\n%s", err, spew.Sdump(a))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(a.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckBeaconAckGUID(a.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleBeaconQuery(id, data []byte) {
	q := client.queryPool.Get().(*protocol.Query)
	defer client.queryPool.Put(q)

	err := msgpack.Unmarshal(data, q)
	if err != nil {
		client.log(logger.Exploit, "invalid beacon query msgpack data:", err)
		client.Close()
		return
	}
	err = q.Validate()
	if err != nil {
		client.logf(logger.Exploit, "invalid beacon query: %s\n%s", err, spew.Sdump(q))
		client.Close()
		return
	}
	expired, timestamp := client.ctx.syncer.CheckGUIDTimestamp(q.GUID)
	if expired {
		client.conn.Reply(id, protocol.ReplyExpired)
		return
	}
	if client.ctx.syncer.CheckQueryGUID(q.GUID, true, timestamp) {
		client.conn.Reply(id, protocol.ReplySucceed)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) Send(guid, message []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, sr.Err = client.conn.Send(protocol.NodeSendGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = client.conn.Send(protocol.NodeSend, message)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

func (client *client) Acknowledge(guid, message []byte) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: client.guid,
	}
	var reply []byte
	reply, ar.Err = client.conn.Send(protocol.NodeAckGUID, guid)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		ar.Err = protocol.GetReplyError(reply)
		return
	}
	reply, ar.Err = client.conn.Send(protocol.NodeAck, message)
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
		_ = client.conn.Close()
		close(client.stopSignal)
		client.wg.Wait()
		if client.closeFunc != nil {
			client.closeFunc()
		}
		client.log(logger.Info, "disconnected")
	})
}

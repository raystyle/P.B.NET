package node

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

	"github.com/pkg/errors"

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
	ctx *Node

	node      *bootstrap.Node
	guid      []byte // node guid
	closeFunc func()

	conn      *conn
	heartbeat chan struct{}
	inSync    int32

	inClose    int32
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

func (client *client) isClosing() bool {
	return atomic.LoadInt32(&client.inClose) != 0
}

func (client *client) onFrame(frame []byte) {
	if client.conn.onFrame(frame) {
		return
	}
	if client.isSync() {
		id := frame[protocol.MsgCMDSize : protocol.MsgCMDSize+protocol.MsgIDSize]
		data := frame[protocol.MsgCMDSize+protocol.MsgIDSize:]
		if client.onFrameAfterSync(frame[0], id, data) {
			return
		}
	}
	switch frame[0] {

	case protocol.ConnReplyHeartbeat:
		select {
		case client.heartbeat <- struct{}{}:
		case <-client.stopSignal:
		}
	default:
		client.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		client.Close()
	}
}

func (client *client) onFrameAfterSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSendToNodeGUID:
		client.handleCtrlSendToNodeGUID(id, data)
	case protocol.CtrlSendToNode:
		// client.handleCtrlSendToNode(id, data)
	case protocol.CtrlAckToNodeGUID:
		client.handleCtrlAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		// client.handleCtrlAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		client.handleCtrlSendToBeaconGUID(id, data)
	case protocol.CtrlSendToBeacon:
		// client.handleCtrlSendToBeacon(id, data)
	case protocol.CtrlAckToBeaconGUID:
		client.handleCtrlAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		// client.handleCtrlAckToBeacon(id, data)
	case protocol.CtrlBroadcastGUID:
		client.handleCtrlBroadcastGUID(id, data)
	case protocol.CtrlBroadcast:
		// client.handleCtrlBroadcast(id, data)
	case protocol.CtrlAnswerGUID:
		client.handleCtrlAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		// client.handleCtrlAnswer(id, data)
	case protocol.NodeSendGUID:
		client.handleNodeSendGUID(id, data)
	case protocol.NodeSend:
		// client.handleNodeSend(id, data)
	case protocol.NodeAckGUID:
		client.handleNodeAckGUID(id, data)
	case protocol.NodeAck:
		// client.handleNodeAck(id, data)
	case protocol.BeaconSendGUID:
		client.handleBeaconSendGUID(id, data)
	case protocol.BeaconSend:
		// client.handleBeaconSend(id, data)
	case protocol.BeaconAckGUID:
		client.handleBeaconAckGUID(id, data)
	case protocol.BeaconAck:
		// client.handleBeaconAck(id, data)
	case protocol.BeaconQueryGUID:
		client.handleBeaconQueryGUID(id, data)
	case protocol.BeaconQuery:
		// client.handleBeaconQuery(id, data)
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

func (client *client) handleCtrlSendToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl send to node guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckCtrlSendToNodeGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleCtrlSendToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl send to beacon guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckCtrlSendToBeaconGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleCtrlAckToNodeGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl ack to node guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckCtrlAckToNodeGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleCtrlAckToBeaconGUID(id, data []byte) {
	if len(data) != guid.Size {
		client.log(logger.Exploit, "invalid ctrl ack to beacon guid size")
		client.conn.Reply(id, protocol.ReplyHandled)
		client.Close()
		return
	}
	if expired, _ := client.ctx.syncer.CheckGUIDTimestamp(data); expired {
		client.conn.Reply(id, protocol.ReplyExpired)
	} else if client.ctx.syncer.CheckCtrlAckToBeaconGUID(data, false, 0) {
		client.conn.Reply(id, protocol.ReplyUnhandled)
	} else {
		client.conn.Reply(id, protocol.ReplyHandled)
	}
}

func (client *client) handleCtrlBroadcastGUID(id, data []byte) {
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

func (client *client) handleCtrlAnswerGUID(id, data []byte) {
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

// Sync is used to switch to sync mode
func (client *client) Sync() error {
	resp, err := client.conn.Send(protocol.NodeSync, nil)
	if err != nil {
		return errors.Wrap(err, "failed to receive sync response")
	}
	if !bytes.Equal(resp, []byte{protocol.NodeSync}) {
		return errors.Errorf("failed to start sync: %s", resp)
	}
	atomic.StoreInt32(&client.inSync, 1)
	return nil
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

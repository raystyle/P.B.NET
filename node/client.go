package node

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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
		conn:      newConn(node.logger, conn, connUsageClient),
	}
	err = client.handshake()
	if err != nil {
		_ = client.conn.Close()
		const format = "failed to handshake with node: %s"
		return nil, errors.WithMessagef(err, format, n.Address)
	}

	return &client, nil
}

// TODO log
func (client *client) log(l logger.Level, log ...interface{}) {
	client.ctx.logger.Print(l, "client", log...)
}

func (client *client) logf(l logger.Level, format string, log ...interface{}) {
	client.ctx.logger.Printf(l, "client", format, log...)
}

func (client *client) handshake() error {
	_ = client.conn.SetDeadline(client.ctx.global.Now().Add(client.ctx.client.Timeout))
	// about check connection
	sizeByte := make([]byte, 1)
	_, err := io.ReadFull(client.conn, sizeByte)
	if err != nil {
		return errors.Wrap(err, "failed to receive check connection size")
	}
	size := int(sizeByte[0])
	checkData := make([]byte, size)
	_, err = io.ReadFull(client.conn, checkData)
	if err != nil {
		return errors.Wrap(err, "failed to receive check connection data")
	}
	_, err = client.conn.Write(random.New().Bytes(size))
	if err != nil {
		return errors.Wrap(err, "failed to send check connection data")
	}
	// receive certificate
	cert, err := client.conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive certificate")
	}
	if !client.verifyCertificate(cert, client.node.Address, client.guid) {
		client.log(logger.Exploit, protocol.ErrInvalidCertificate)
		return protocol.ErrInvalidCertificate
	}
	// send role
	_, err = client.conn.Write(protocol.Node.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send role")
	}
	// send self guid
	_, err = client.conn.Write(client.ctx.global.GUID())
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
				client.log(logger.Fatal, xpanic.Error(r, "client:"))
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
	// check command
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
	case protocol.NodeSendGUID:

	case protocol.NodeSend:

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

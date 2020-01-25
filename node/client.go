package node

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/rand"
	"project/internal/dns"
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
	guid      []byte
	closeFunc func()

	tag       string
	Conn      *conn
	rand      *random.Rand
	heartbeat chan struct{}
	inSync    int32
	syncM     sync.Mutex

	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// newClient is used to create a client that connected Node
// when guid != ctrl guid for forwarder
// when guid == ctrl guid for register
func (node *Node) newClient(
	ctx context.Context,
	n *bootstrap.Node,
	guid []byte,
	closeFunc func(),
) (*client, error) {
	// dial
	host, port, err := net.SplitHostPort(n.Address)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	opts := xnet.Options{
		TLSConfig: &tls.Config{
			Rand:       rand.Reader,
			Time:       node.global.Now,
			ServerName: host,
			RootCAs:    x509.NewCertPool(),
			MinVersion: tls.VersionTLS12,
		},
		Timeout: node.clientMgr.GetTimeout(),
		Now:     node.global.Now,
	}
	// add CA certificates
	for _, cert := range node.global.Certificates() {
		opts.TLSConfig.RootCAs.AddCert(cert)
	}
	// set proxy
	proxy, err := node.global.GetProxyClient(node.clientMgr.GetProxyTag())
	if err != nil {
		return nil, err
	}
	opts.Dialer = proxy.DialContext
	// resolve domain name
	dnsOpts := node.clientMgr.GetDNSOptions()
	result, err := node.global.ResolveWithContext(ctx, host, dnsOpts)
	if err != nil {
		return nil, err
	}
	var conn *xnet.Conn
	for i := 0; i < len(result); i++ {
		address := net.JoinHostPort(result[i], port)
		conn, err = xnet.DialContext(ctx, n.Mode, n.Network, address, &opts)
		if err == nil {
			break
		}
	}
	if conn == nil {
		return nil, errors.Errorf("failed to connect node: %s", n.Address)
	}

	// handshake
	client := &client{
		ctx:       node,
		node:      n,
		guid:      guid,
		closeFunc: closeFunc,
		rand:      random.New(),
	}
	client.Conn = newConn(node, conn, guid, connUsageClient)
	err = client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		const format = "failed to handshake with node: %s"
		return nil, errors.WithMessagef(err, format, n.Address)
	}

	// add client to client manager
	client.tag = node.clientMgr.GenerateTag()
	node.clientMgr.Add(client)
	return client, nil
}

func (client *client) handshake(conn *xnet.Conn) error {
	timeout := client.ctx.clientMgr.GetTimeout()
	_ = conn.SetDeadline(client.ctx.global.Now().Add(timeout))
	// about check connection
	err := client.checkConn(conn)
	if err != nil {
		return err
	}
	// receive certificate
	cert, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive certificate")
	}
	if !client.verifyCertificate(cert, client.node.Address, client.guid) {
		client.Conn.Log(logger.Exploit, protocol.ErrInvalidCertificate)
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

func (client *client) checkConn(conn *xnet.Conn) error {
	size := byte(100 + client.rand.Int(156))
	data := client.rand.Bytes(int(size))
	_, err := conn.Write(append([]byte{size}, data...))
	if err != nil {
		return errors.WithMessage(err, "failed to send check connection data")
	}
	n, err := io.ReadFull(conn, data)
	if err != nil {
		d := data[:n]
		const format = "error in client.checkConn(): %s\nreceive unexpected check data\n%s\n\n%X"
		client.Conn.Logf(logger.Exploit, format, err, d, d)
		return err
	}
	return nil
}

func (client *client) verifyCertificate(cert []byte, address string, guid []byte) bool {
	if len(cert) != 2*ed25519.SignatureSize {
		return false
	}
	// verify certificate
	buffer := bytes.Buffer{}
	buffer.WriteString(address)
	buffer.Write(guid)
	if bytes.Compare(guid, protocol.CtrlGUID) == 0 {
		certWithCtrlGUID := cert[ed25519.SignatureSize:]
		return client.ctx.global.CtrlVerify(buffer.Bytes(), certWithCtrlGUID)
	}
	certWithNodeGUID := cert[:ed25519.SignatureSize]
	return client.ctx.global.CtrlVerify(buffer.Bytes(), certWithNodeGUID)
}

func (client *client) Connect() (err error) {
	defer func() {
		if err != nil {
			client.Close()
		}
	}()
	// send connect operation
	_, err = client.Conn.Write([]byte{nodeOperationConnect})
	if err != nil {
		err = errors.Wrap(err, "failed to send connect operation")
		return
	}
	err = client.authenticate()
	if err != nil {
		return
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
				client.Conn.Log(logger.Fatal, xpanic.Print(r, "client.HandleConn"))
			}
			client.Close()
		}()
		protocol.HandleConn(client.Conn, client.onFrame)
	}()
	return
}

func (client *client) authenticate() error {
	// receive challenge
	challenge, err := client.Conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive challenge")
	}
	if len(challenge) < 2048 || len(challenge) > 4096 {
		err = errors.New("invalid challenge size")
		client.Conn.Log(logger.Exploit, err)
		return err
	}
	// send signature
	err = client.Conn.SendMessage(client.ctx.global.Sign(challenge))
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

func (client *client) isSync() bool {
	return atomic.LoadInt32(&client.inSync) != 0
}

func (client *client) onFrame(frame []byte) {
	if client.Conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnReplyHeartbeat {
		select {
		case client.heartbeat <- struct{}{}:
		case <-client.stopSignal:
		}
	}
	if client.isSync() {
		if client.onFrameAfterSync(frame) {
			return
		}
	}
	const format = "unknown command: %d\nframe:\n%s"
	client.Conn.Logf(logger.Exploit, format, frame[0], spew.Sdump(frame))
	client.Close()
}

func (client *client) onFrameAfterSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	if client.onFrameAfterSyncAboutCTRL(frame[0], id, data) {
		return true
	}
	if client.onFrameAfterSyncAboutNode(frame[0], id, data) {
		return true
	}
	if client.onFrameAfterSyncAboutBeacon(frame[0], id, data) {
		return true
	}
	return false
}

func (client *client) onFrameAfterSyncAboutCTRL(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSendToNodeGUID:
		client.Conn.HandleSendToNodeGUID(id, data)
	case protocol.CtrlSendToNode:
		client.Conn.HandleSendToNode(id, data)
	case protocol.CtrlAckToNodeGUID:
		client.Conn.HandleAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		client.Conn.HandleAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		client.Conn.HandleSendToBeaconGUID(id, data)
	case protocol.CtrlSendToBeacon:
		client.Conn.HandleSendToBeacon(id, data)
	case protocol.CtrlAckToBeaconGUID:
		client.Conn.HandleAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		client.Conn.HandleAckToBeacon(id, data)
	case protocol.CtrlBroadcastGUID:
		client.Conn.HandleBroadcastGUID(id, data)
	case protocol.CtrlBroadcast:
		client.Conn.HandleBroadcast(id, data)
	case protocol.CtrlAnswerGUID:
		client.Conn.HandleAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		client.Conn.HandleAnswer(id, data)
	default:
		return false
	}
	return true
}

func (client *client) onFrameAfterSyncAboutNode(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.NodeSendGUID:
		client.Conn.HandleNodeSendGUID(id, data)
	case protocol.NodeSend:
		client.Conn.HandleNodeSend(id, data)
	case protocol.NodeAckGUID:
		client.Conn.HandleNodeAckGUID(id, data)
	case protocol.NodeAck:
		client.Conn.HandleNodeAck(id, data)
	default:
		return false
	}
	return true
}

func (client *client) onFrameAfterSyncAboutBeacon(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.BeaconSendGUID:
		client.Conn.HandleBeaconSendGUID(id, data)
	case protocol.BeaconSend:
		client.Conn.HandleBeaconSend(id, data)
	case protocol.BeaconAckGUID:
		client.Conn.HandleBeaconAckGUID(id, data)
	case protocol.BeaconAck:
		client.Conn.HandleBeaconAck(id, data)
	case protocol.BeaconQueryGUID:
		client.Conn.HandleBeaconQueryGUID(id, data)
	case protocol.BeaconQuery:
		client.Conn.HandleBeaconQuery(id, data)
	default:
		return false
	}
	return true
}

func (client *client) sendHeartbeatLoop() {
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
				client.Conn.Log(logger.Warning, "receive heartbeat timeout")
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

// Synchronize is used to switch to synchronize mode
func (client *client) Synchronize() error {
	client.syncM.Lock()
	defer client.syncM.Unlock()
	if client.isSync() {
		return nil
	}
	resp, err := client.Conn.SendCommand(protocol.NodeSync, nil)
	if err != nil {
		return errors.Wrap(err, "failed to receive synchronize response")
	}
	if bytes.Compare(resp, []byte{protocol.NodeSync}) != 0 {
		return errors.Errorf("failed to start synchronize: %s", resp)
	}
	// initialize sync pool
	client.Conn.SendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	client.Conn.AckPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	client.Conn.AnswerPool.New = func() interface{} {
		return &protocol.Answer{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Message:    make([]byte, aes.BlockSize),
			Hash:       make([]byte, sha256.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	client.Conn.QueryPool.New = func() interface{} {
		return &protocol.Query{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	// TODO register
	// client.ctx.forwarder.RegisterNode(client)
	atomic.StoreInt32(&client.inSync, 1)
	return nil
}

// Status is used to get connection status
func (client *client) Status() *xnet.Status {
	return client.Conn.Status()
}

// Close is used to disconnect node
func (client *client) Close() {
	client.closeOnce.Do(func() {
		_ = client.Conn.Close()
		close(client.stopSignal)
		client.wg.Wait()
		client.ctx.clientMgr.Delete(client.tag)
		if client.closeFunc != nil {
			client.closeFunc()
		}
		client.Conn.Log(logger.Info, "disconnected")
	})
}

// clientMgr contains all clients from newClient() and client options from Config
// it can generate client tag, you can manage all clients here
type clientMgr struct {
	ctx *Node

	// options from Config
	proxyTag string
	timeout  time.Duration
	dnsOpts  dns.Options
	optsRWM  sync.RWMutex

	guid       *guid.Generator
	clients    map[string]*client
	clientsRWM sync.RWMutex
}

func newClientManager(ctx *Node, config *Config) (*clientMgr, error) {
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
		clients:  make(map[string]*client),
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

// GenerateTag is used to generate client tag, it for newClient()
func (cm *clientMgr) GenerateTag() string {
	return hex.EncodeToString(cm.guid.Get())
}

// for newClient()
func (cm *clientMgr) Add(client *client) {
	cm.clientsRWM.Lock()
	defer cm.clientsRWM.Unlock()
	if _, ok := cm.clients[client.tag]; !ok {
		cm.clients[client.tag] = client
	}
}

// for client.Close()
func (cm *clientMgr) Delete(tag string) {
	cm.clientsRWM.Lock()
	defer cm.clientsRWM.Unlock()
	delete(cm.clients, tag)
}

// Clients is used to get all clients
func (cm *clientMgr) Clients() map[string]*client {
	cm.clientsRWM.RLock()
	defer cm.clientsRWM.RUnlock()
	cs := make(map[string]*client, len(cm.clients))
	for tag, client := range cm.clients {
		cs[tag] = client
	}
	return cs
}

// Kill is used to close client
func (cm *clientMgr) Kill(tag string) {
	// must use cm.Clients(), because client.Close() will use cm.clientsRWM
	if client, ok := cm.Clients()[tag]; ok {
		client.Close()
	}
}

func (cm *clientMgr) Close() {
	cm.guid.Close()
	cm.ctx = nil
}

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

// Client is used to connect node listener
type Client struct {
	ctx *Node

	listener *bootstrap.Listener
	GUID     *guid.GUID // Node GUID

	tag       *guid.GUID
	Conn      *conn
	rand      *random.Rand
	heartbeat chan struct{}
	inSync    int32
	syncM     sync.Mutex

	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// NewClient is used to create a client and connect node listener
// when guid != ctrl guid for forwarder
// when guid == ctrl guid for register
func (node *Node) NewClient(
	ctx context.Context,
	bl *bootstrap.Listener,
	guid *guid.GUID,
) (*Client, error) {
	// dial
	host, port, err := net.SplitHostPort(bl.Address)
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
		ctx:      node,
		listener: bl,
		GUID:     guid,
		tag:      node.clientMgr.GenerateTag(),
		rand:     random.New(),
	}
	client.Conn = newConn(node, conn, guid, connUsageClient)
	err = client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		const format = "failed to handshake with node listener: %s"
		return nil, errors.WithMessagef(err, format, bl)
	}
	node.clientMgr.Add(client)
	client.Conn.Log(logger.Info, "create client")
	return client, nil
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
	ok, err := protocol.VerifyCertificate(conn, publicKey, client.GUID)
	if err != nil {
		client.Conn.Log(logger.Exploit, err)
		return err
	}
	if !ok {
		return errors.New("failed to verify node certificate")
	}
	// send role
	_, err = conn.Write(protocol.Node.Bytes())
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
		client.Conn.Logf(logger.Exploit, format, err, spew.Sdump(data[:n]))
		return err
	}
	return nil
}

// Connect is used to start protocol.HandleConn(), if you want to
// start Synchronize(), you must call this function first
func (client *Client) Connect() (err error) {
	defer func() {
		if err != nil {
			client.Close()
		}
	}()
	// send connect operation
	_, err = client.Conn.Write([]byte{protocol.NodeOperationConnect})
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
			// logoff forwarder
			client.syncM.Lock()
			defer client.syncM.Unlock()
			if client.isSync() {
				client.ctx.forwarder.LogoffClient(client.GUID)
			}
			client.Close()
		}()
		protocol.HandleConn(client.Conn, client.onFrame)
	}()
	timeout := client.ctx.clientMgr.GetTimeout()
	_ = client.Conn.SetDeadline(client.ctx.global.Now().Add(timeout))
	client.Conn.Log(logger.Debug, "connected")
	return
}

func (client *Client) authenticate() error {
	// receive challenge
	challenge, err := client.Conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive challenge")
	}
	if len(challenge) != protocol.ChallengeSize {
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
				_ = client.Conn.Close()
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

func (client *Client) isSync() bool {
	return atomic.LoadInt32(&client.inSync) != 0
}

func (client *Client) onFrame(frame []byte) {
	if client.Conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnReplyHeartbeat {
		select {
		case client.heartbeat <- struct{}{}:
		case <-client.stopSignal:
		}
		return
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

// Synchronize is used to switch to synchronize mode
func (client *Client) Synchronize() error {
	client.syncM.Lock()
	defer client.syncM.Unlock()
	if client.isSync() {
		return errors.New("already synchronize")
	}
	// initialize sync pool
	client.Conn.SendPool.New = func() interface{} {
		return protocol.NewSend()
	}
	client.Conn.AckPool.New = func() interface{} {
		return protocol.NewAcknowledge()
	}
	client.Conn.AnswerPool.New = func() interface{} {
		return protocol.NewAnswer()
	}
	client.Conn.QueryPool.New = func() interface{} {
		return protocol.NewQuery()
	}
	// must presume, or may be lost message.
	var err error
	atomic.StoreInt32(&client.inSync, 1)
	defer func() {
		if err != nil {
			atomic.StoreInt32(&client.inSync, 0)
		}
	}()
	resp, err := client.Conn.SendCommand(protocol.NodeSync, nil)
	if err != nil {
		return errors.Wrap(err, "failed to receive synchronize response")
	}
	if bytes.Compare(resp, []byte{protocol.NodeSync}) != 0 {
		err = errors.Errorf("failed to start synchronize: %s", resp)
		return err // can't return directly
	}
	err = client.ctx.forwarder.RegisterClient(client.GUID, client)
	if err != nil {
		return err
	}
	client.Conn.Logf(logger.Info, "start synchronize\nlistener: %s", client.listener)
	return nil
}

func (client *Client) onFrameAfterSync(frame []byte) bool {
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

func (client *Client) onFrameAfterSyncAboutCTRL(cmd byte, id, data []byte) bool {
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

func (client *Client) onFrameAfterSyncAboutNode(cmd byte, id, data []byte) bool {
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

func (client *Client) onFrameAfterSyncAboutBeacon(cmd byte, id, data []byte) bool {
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

// Status is used to get connection status
func (client *Client) Status() *xnet.Status {
	return client.Conn.Status()
}

// Close is used to disconnect node
func (client *Client) Close() {
	client.closeOnce.Do(func() {
		_ = client.Conn.Close()
		if client.stopSignal != nil {
			close(client.stopSignal)
		}
		client.wg.Wait()
		client.ctx.clientMgr.Delete(client.tag)
		client.Conn.Log(logger.Info, "disconnected")
	})
}

// clientMgr contains all clients from NewClient() and client options from Config
// it can generate client tag, you can manage all clients here
type clientMgr struct {
	ctx *Node

	// options from Config
	proxyTag string
	timeout  time.Duration
	dnsOpts  dns.Options
	optsRWM  sync.RWMutex

	guid       *guid.Generator
	clients    map[guid.GUID]*Client
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
		guid:     ctx.global.GetGUIDGenerator(),
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
func (cm *clientMgr) GenerateTag() *guid.GUID {
	return cm.guid.Get()
}

// for NewClient()
func (cm *clientMgr) Add(client *Client) {
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

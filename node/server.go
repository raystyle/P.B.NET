package node

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/rand"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/nettool"
	"project/internal/patch/msgpack"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xnet"
	"project/internal/xpanic"
)

// ErrServerClosed is returned by the server's Serve, AddListener
// methods after a call Close
var ErrServerClosed = fmt.Errorf("server closed")

// server is used to accept beacon node controller connections
type server struct {
	ctx *Node

	maxConns int           // each listener
	timeout  time.Duration // handshake timeout

	guid *guid.Generator
	rand *random.Rand

	// key = listener tag
	listeners  map[string]*xnet.Listener
	conns      map[guid.GUID]*xnet.Conn
	inShutdown int32
	rwm        sync.RWMutex

	// key = connection tag
	ctrlConns      map[guid.GUID]*ctrlConn
	ctrlConnsRWM   sync.RWMutex
	nodeConns      map[guid.GUID]*nodeConn
	nodeConnsRWM   sync.RWMutex
	beaconConns    map[guid.GUID]*beaconConn
	beaconConnsRWM sync.RWMutex

	context   context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func newServer(ctx *Node, config *Config) (*server, error) {
	cfg := config.Server

	if cfg.MaxConns < 1 {
		return nil, errors.New("listener max connection must > 0")
	}
	if cfg.Timeout < 15*time.Second {
		return nil, errors.New("listener max timeout must >= 15s")
	}

	memory := security.NewMemory()
	defer memory.Flush()

	server := server{
		ctx:         ctx,
		maxConns:    cfg.MaxConns,
		timeout:     cfg.Timeout,
		guid:        guid.New(4, ctx.global.Now),
		rand:        random.New(),
		listeners:   make(map[string]*xnet.Listener),
		conns:       make(map[guid.GUID]*xnet.Conn),
		ctrlConns:   make(map[guid.GUID]*ctrlConn),
		nodeConns:   make(map[guid.GUID]*nodeConn),
		beaconConns: make(map[guid.GUID]*beaconConn),
	}
	server.context, server.cancel = context.WithCancel(context.Background())

	// decrypt listeners configs
	if len(cfg.Listeners) != 0 {
		if len(cfg.ListenersKey) != aes.Key256Bit+aes.IVSize {
			return nil, errors.New("invalid aes key size")
		}
		aesKey := cfg.ListenersKey[:aes.Key256Bit]
		aesIV := cfg.ListenersKey[aes.Key256Bit:]
		defer func() {
			security.CoverBytes(aesKey)
			security.CoverBytes(aesIV)
		}()
		memory.Padding()
		data, err := aes.CBCDecrypt(cfg.Listeners, aesKey, aesIV)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		security.CoverBytes(aesKey)
		security.CoverBytes(aesIV)
		memory.Padding()
		var listeners []*messages.Listener
		err = msgpack.Unmarshal(data, &listeners)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		for i := 0; i < len(listeners); i++ {
			memory.Padding()
			_, err = server.addListener(listeners[i])
			if err != nil {
				return nil, err
			}
		}
	}
	return &server, nil
}

// Deploy is used to deploy added listener
func (server *server) Deploy() error {
	// deploy all listener
	l := len(server.listeners)
	errs := make(chan error, l)
	for tag, listener := range server.listeners {
		go func(tag string, listener *xnet.Listener) {
			errs <- server.deploy(tag, listener)
		}(tag, listener)
	}
	for i := 0; i < l; i++ {
		err := <-errs
		if err != nil {
			return err
		}
	}
	return nil
}

func (server *server) logf(lv logger.Level, format string, log ...interface{}) {
	server.ctx.logger.Printf(lv, "server", format, log...)
}

func (server *server) log(lv logger.Level, log ...interface{}) {
	server.ctx.logger.Println(lv, "server", log...)
}

func (server *server) addListener(l *messages.Listener) (*xnet.Listener, error) {
	failed := func(err error) error {
		return errors.WithMessagef(err, "failed to add listener %s", l.Tag)
	}
	// disable client certificates
	l.TLSConfig.CertPool = server.ctx.global.CertPool
	l.TLSConfig.ServerSide = true
	// apply tls config
	tlsConfig, err := l.TLSConfig.Apply()
	if err != nil {
		return nil, failed(err)
	}
	// <security>
	tlsConfig.Rand = rand.Reader
	tlsConfig.Time = server.ctx.global.Now
	// fake nginx server
	if len(tlsConfig.NextProtos) == 0 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	opts := xnet.Options{
		TLSConfig: tlsConfig,
		Timeout:   l.Timeout,
		Now:       server.ctx.global.Now,
	}
	listener, err := xnet.Listen(l.Mode, l.Network, l.Address, &opts)
	if err != nil {
		return nil, failed(err)
	}
	// add limit connections
	listener.Listener = netutil.LimitListener(listener.Listener, server.maxConns)

	server.rwm.Lock()
	defer server.rwm.Unlock()
	if _, ok := server.listeners[l.Tag]; !ok {
		server.listeners[l.Tag] = listener
		return listener, nil
	}
	return nil, errors.Errorf("listener %s already exists", l.Tag)
}

func (server *server) deploy(tag string, listener *xnet.Listener) error {
	errChan := make(chan error, 1)
	server.wg.Add(1)
	go server.serve(tag, listener, errChan)
	select {
	case err := <-errChan:
		const format = "failed to add listener %s(%s): %s"
		return errors.Errorf(format, tag, listener.Addr(), err)
	case <-time.After(time.Second):
		network := listener.Addr().Network()
		address := listener.Addr().String()
		const format = "add listener: %s %s (%s %s)"
		server.logf(logger.Info, format, tag, listener.Mode(), network, address)
		return nil
	}
}

func (server *server) serve(tag string, l *xnet.Listener, errChan chan<- error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "server.serve()")
			server.log(logger.Fatal, err)
		}
		errChan <- err
		close(errChan)
		// delete
		server.rwm.Lock()
		defer server.rwm.Unlock()
		delete(server.listeners, tag)
		addr := l.Addr()
		network := addr.Network()
		server.logf(logger.Info, "listener: %s (%s %s) is closed", tag, network, addr)
		server.wg.Done()
	}()
	var delay time.Duration // how long to sleep on accept failure
	maxDelay := 2 * time.Second
	for {
		conn, e := l.AcceptEx()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if delay > maxDelay {
					delay = maxDelay
				}
				server.logf(logger.Warning, "acceptEx error: %s; retrying in %v", e, delay)
				time.Sleep(delay)
				continue
			}
			errStr := e.Error()
			if !strings.Contains(errStr, "closed") &&
				!strings.Contains(errStr, "context canceled") {
				server.logf(logger.Warning, "acceptEx error: %s", errStr)
				err = e
			}
			return
		}
		delay = 0
		server.wg.Add(1)
		go server.handshake(conn)
	}
}

func (server *server) shuttingDown() bool {
	return atomic.LoadInt32(&server.inShutdown) != 0
}

func (server *server) AddListener(l *messages.Listener) error {
	if server.shuttingDown() {
		return errors.WithStack(ErrServerClosed)
	}
	listener, err := server.addListener(l)
	if err != nil {
		return err
	}
	return server.deploy(l.Tag, listener)
}

func (server *server) Listeners() map[string]*xnet.Listener {
	server.rwm.RLock()
	defer server.rwm.RUnlock()
	listeners := make(map[string]*xnet.Listener, len(server.listeners))
	for tag, listener := range server.listeners {
		listeners[tag] = listener
	}
	return listeners
}

func (server *server) GetListener(tag string) (*xnet.Listener, error) {
	server.rwm.RLock()
	defer server.rwm.RUnlock()
	if listener, ok := server.listeners[tag]; ok {
		return listener, nil
	}
	return nil, errors.Errorf("listener %s doesn't exists", tag)
}

func (server *server) CloseListener(tag string) error {
	server.rwm.RLock()
	defer server.rwm.RUnlock()
	if listener, ok := server.listeners[tag]; ok {
		return listener.Close()
	}
	return errors.Errorf("listener %s doesn't exists", tag)
}

func (server *server) Conns() map[guid.GUID]*xnet.Conn {
	server.rwm.RLock()
	defer server.rwm.RUnlock()
	conns := make(map[guid.GUID]*xnet.Conn, len(server.conns))
	for tag, conn := range server.conns {
		conns[tag] = conn
	}
	return conns
}

func (server *server) GetConn(tag *guid.GUID) (*xnet.Conn, error) {
	server.rwm.RLock()
	defer server.rwm.RUnlock()
	if conn, ok := server.conns[*tag]; ok {
		return conn, nil
	}
	return nil, errors.Errorf("conn doesn't exists\n%s", tag)
}

func (server *server) CloseConn(tag *guid.GUID) error {
	server.rwm.RLock()
	defer server.rwm.RUnlock()
	if conn, ok := server.conns[*tag]; ok {
		return conn.Close()
	}
	return errors.Errorf("conn doesn't exists\n%s", tag)
}

// Close is used to close all listeners and connections
func (server *server) Close() {
	server.closeOnce.Do(func() {
		server.cancel()
		atomic.StoreInt32(&server.inShutdown, 1)
		server.rwm.Lock()
		defer server.rwm.Unlock()
		// close all listeners
		for _, listener := range server.listeners {
			_ = listener.Close()
		}
		// close all connections
		for _, conn := range server.conns {
			_ = conn.Close()
		}
		server.guid.Close()
	})
	server.wg.Wait()
	server.ctx = nil
}

func (server *server) logfConn(c *xnet.Conn, lv logger.Level, format string, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, format+"\n", log...)
	const conn = "---------------connection status----------------\n%s\n"
	_, _ = fmt.Fprintf(buf, conn, c)
	const endLine = "------------------------------------------------"
	_, _ = fmt.Fprint(buf, endLine)
	server.ctx.logger.Print(lv, "server", buf)
}

func (server *server) logConn(c *xnet.Conn, lv logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log...)
	const conn = "---------------connection status----------------\n%s\n"
	_, _ = fmt.Fprintf(buf, conn, c)
	const endLine = "------------------------------------------------"
	_, _ = fmt.Fprint(buf, endLine)
	server.ctx.logger.Print(lv, "server", buf)
}

func (server *server) addConn(tag *guid.GUID, conn *xnet.Conn) {
	server.rwm.Lock()
	defer server.rwm.Unlock()
	server.conns[*tag] = conn
}

func (server *server) deleteConn(tag *guid.GUID) {
	server.rwm.Lock()
	defer server.rwm.Unlock()
	delete(server.conns, *tag)
}

func (server *server) addCtrlConn(tag *guid.GUID, conn *ctrlConn) {
	server.ctrlConnsRWM.Lock()
	defer server.ctrlConnsRWM.Unlock()
	if _, ok := server.ctrlConns[*tag]; !ok {
		server.ctrlConns[*tag] = conn
	}
}

func (server *server) deleteCtrlConn(tag *guid.GUID) {
	server.ctrlConnsRWM.Lock()
	defer server.ctrlConnsRWM.Unlock()
	delete(server.ctrlConns, *tag)
}

func (server *server) addNodeConn(tag *guid.GUID, conn *nodeConn) {
	server.nodeConnsRWM.Lock()
	defer server.nodeConnsRWM.Unlock()
	if _, ok := server.nodeConns[*tag]; !ok {
		server.nodeConns[*tag] = conn
	}
}

func (server *server) deleteNodeConn(tag *guid.GUID) {
	server.nodeConnsRWM.Lock()
	defer server.nodeConnsRWM.Unlock()
	delete(server.nodeConns, *tag)
}

func (server *server) addBeaconConn(tag *guid.GUID, conn *beaconConn) {
	server.beaconConnsRWM.Lock()
	defer server.beaconConnsRWM.Unlock()
	if _, ok := server.beaconConns[*tag]; !ok {
		server.beaconConns[*tag] = conn
	}
}

func (server *server) deleteBeaconConn(tag *guid.GUID) {
	server.beaconConnsRWM.Lock()
	defer server.beaconConnsRWM.Unlock()
	delete(server.beaconConns, *tag)
}

func (server *server) handshake(conn *xnet.Conn) {
	defer func() {
		if r := recover(); r != nil {
			server.logConn(conn, logger.Exploit, xpanic.Print(r, "server.handshake"))
		}
		_ = conn.Close()
		server.wg.Done()
	}()
	// add to server.conns for management
	tag := server.guid.Get()
	server.addConn(tag, conn)
	defer server.deleteConn(tag)
	// check connection and send certificate
	_ = conn.SetDeadline(time.Now().Add(server.timeout))
	if !server.checkConn(conn) {
		return
	}
	if !server.sendCertificate(conn) {
		return
	}
	// receive challenge and sign it
	challenge := make([]byte, protocol.ChallengeSize)
	_, err := io.ReadFull(conn, challenge)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to receive challenge")
		return
	}
	_, err = conn.Write(server.ctx.global.Sign(challenge))
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send challenge signature")
		return
	}
	// receive role
	r := make([]byte, 1)
	_, err = io.ReadFull(conn, r)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to receive role")
		return
	}
	role := protocol.Role(r[0])
	switch role {
	case protocol.Ctrl:
		server.handshakeWithCtrl(tag, conn)
	case protocol.Node:
		server.handshakeWithNode(tag, conn)
	case protocol.Beacon:
		server.handshakeWithBeacon(tag, conn)
	default:
		server.logConn(conn, logger.Exploit, role)
	}
}

// checkConn is used to check connection is from client.
// If read http request, return a fake http response.
func (server *server) checkConn(conn *xnet.Conn) bool {
	// read generated random data size
	size := make([]byte, 1)
	_, err := io.ReadFull(conn, size)
	if err != nil {
		const format = "failed to check connection\n%s"
		server.logfConn(conn, logger.Error, format, err)
		return false
	}
	// read random data
	randomData := make([]byte, size[0])
	n, err := io.ReadFull(conn, randomData)
	total := append(size, randomData[:n]...)
	if err != nil {
		const format = "receive test data in checkConn\n%s\n\n%X"
		server.logfConn(conn, logger.Error, format, total, total)
		return false
	}
	if server.isHTTPRequest(total, conn) {
		return false
	}
	// write generated random data
	_, err = conn.Write(server.rand.Bytes(int(size[0])))
	return err == nil
}

var nginxBody = strings.ReplaceAll(`<html>
<head><title>403 Forbidden</title></head>
<body>
<center><h1>403 Forbidden</h1></center>
<hr><center>nginx</center>
</body>
</html>
`, "\n", "\r\n")

func (server *server) isHTTPRequest(data []byte, conn *xnet.Conn) bool {
	// check is http request
	lines := strings.Split(string(data), "\r\n")
	// GET / HTTP/1.1
	rl := strings.Split(lines[0], " ") // request line
	if len(rl) != 3 {
		return false
	}
	if !strings.Contains(rl[2], "HTTP") {
		return false
	}
	// read rest data
	go func() {
		defer func() {
			if r := recover(); r != nil {
				b := xpanic.Print(r, "server.isHTTPRequest")
				server.logConn(conn, logger.Error, b)
			}
		}()
		_, _ = io.Copy(ioutil.Discard, conn)
	}()
	// write 403 response
	buf := new(bytes.Buffer)
	// status line
	_, _ = fmt.Fprintf(buf, "%s 403 Forbidden\r\n", rl[2])
	// fake nginx server
	buf.WriteString("Server: nginx\r\n")
	// write date
	date := server.ctx.global.Now().Local().Format(http.TimeFormat)
	_, _ = fmt.Fprintf(buf, "Date: %s\r\n", date)
	// other
	buf.WriteString("Content-Type: text/html\r\n")
	_, _ = fmt.Fprintf(buf, "Content-Length: %d\r\n", len(nginxBody))
	buf.WriteString("Connection: keep-alive\r\n\r\n")
	buf.WriteString(nginxBody)
	_, _ = io.Copy(conn, buf)
	return true
}

func (server *server) sendCertificate(conn *xnet.Conn) bool {
	var err error
	cert := server.ctx.global.GetCertificate()
	if cert != nil {
		_, err = conn.Write(cert)
	} else { // if no certificate, send random certificate with Node GUID and public key
		cert := protocol.Certificate{
			GUID:      *server.ctx.global.GUID(),
			PublicKey: server.ctx.global.PublicKey(),
		}
		cert.Signatures[0] = server.rand.Bytes(ed25519.SignatureSize)
		cert.Signatures[1] = server.rand.Bytes(ed25519.SignatureSize)
		_, err = conn.Write(cert.Encode())
	}
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send certificate:", err)
		return false
	}
	return true
}

func (server *server) handshakeWithCtrl(tag *guid.GUID, conn *xnet.Conn) {
	// <danger>
	// maybe fake node will send some special data
	// and controller sign it
	challenge := server.rand.Bytes(protocol.ChallengeSize)
	err := conn.Send(challenge)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send challenge to controller:", err)
		return
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		server.logConn(conn, logger.Error, "failed to receive controller signature:", err)
		return
	}
	// verify signature
	if !server.ctx.global.CtrlVerify(challenge, signature) {
		server.logConn(conn, logger.Exploit, "invalid controller signature")
		return
	}
	// send succeed response
	err = conn.Send(protocol.AuthSucceed)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send response to controller:", err)
		return
	}
	server.serveCtrl(tag, conn)
}

func (server *server) handshakeWithNode(tag *guid.GUID, conn *xnet.Conn) {
	nodeGUID := guid.GUID{}
	_, err := io.ReadFull(conn, nodeGUID[:])
	if err != nil {
		server.logConn(conn, logger.Error, "failed to receive node guid:", err)
		return
	}
	// check is self
	if nodeGUID == *server.ctx.global.GUID() {
		server.logConn(conn, logger.Warning, "oh! self")
		return
	}
	// read operation
	operation := make([]byte, 1)
	_, err = io.ReadFull(conn, operation)
	if err != nil {
		server.logConn(conn, logger.Exploit, "failed to receive node operation", err)
		return
	}
	switch operation[0] {
	case protocol.NodeOperationRegister: // register
		server.registerNode(conn, &nodeGUID)
	case protocol.NodeOperationConnect: // connect
		if !server.verifyNode(conn, &nodeGUID) {
			return
		}
		server.serveNode(tag, &nodeGUID, conn)
	default:
		server.logfConn(conn, logger.Exploit, "unknown node operation %d", operation[0])
	}
}

func (server *server) registerNode(conn *xnet.Conn, guid *guid.GUID) {
	// send external address
	err := conn.Send(nettool.EncodeExternalAddress(conn.RemoteAddr().String()))
	if err != nil {
		const log = "failed to send node external address:"
		server.logConn(conn, logger.Error, log, err)
		return
	}
	// Node send self key exchange public key (curve25519),
	// use session key encrypt register request data.
	//
	// +----------------+----------------+
	// | kex public key | encrypted data |
	// +----------------+----------------+
	// |    32 Bytes    |       var      |
	// +----------------+----------------+
	//
	// receive encrypted Node register request
	request, err := conn.Receive()
	if err != nil {
		const log = "failed to receive node register request:"
		server.logConn(conn, logger.Error, log, err)
		return
	}
	// check size
	if len(request) < curve25519.ScalarSize+aes.BlockSize {
		const log = "receive invalid encrypted node register request"
		server.logConn(conn, logger.Exploit, log)
		return
	}
	// create node register
	response := server.ctx.storage.CreateNodeRegister(guid)
	if response == nil {
		const format = "failed to create node register\nGUID: %X"
		server.logfConn(conn, logger.Warning, format, guid)
		return
	}
	// <security> must don't handle error.
	_ = server.ctx.sender.Send(server.context,
		messages.CMDBNodeRegisterRequest, request, true)
	// wait register result
	timeout := time.Duration(15+server.rand.Int(30)) * time.Second
	timer := time.AfterFunc(timeout, func() {
		defer func() {
			if r := recover(); r != nil {
				server.log(logger.Fatal, xpanic.Print(r, "server.registerNode"))
			}
		}()
		server.ctx.storage.SetNodeRegister(guid, &messages.NodeRegisterResponse{
			Result: messages.RegisterResultTimeout,
		})
	})
	defer timer.Stop()
	// read register response result
	var resp *messages.NodeRegisterResponse
	select {
	case resp = <-response:
	case <-server.context.Done():
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(server.timeout))
	switch resp.Result {
	case messages.RegisterResultAccept:
		_, _ = conn.Write([]byte{messages.RegisterResultAccept})
		_, _ = conn.Write(resp.Certificate)
		_ = conn.Send(resp.NodeListeners)
	case messages.RegisterResultRefused:
		// TODO add IP black list only register(other role still pass)
		// and <firewall> rate limit

		_, _ = conn.Write([]byte{messages.RegisterResultTimeout})
	case messages.RegisterResultTimeout:
		_, _ = conn.Write([]byte{messages.RegisterResultTimeout})
	default:
		const format = "unknown node register result: %d"
		server.logfConn(conn, logger.Exploit, format, resp.Result)
	}
}

func (server *server) getNodeKey(guid *guid.GUID) *protocol.NodeKey {
	// First try to query from self storage.
	nk := server.ctx.storage.GetNodeKey(guid)
	if nk != nil {
		return nk
	}
	// If doesn't exists in self storage, try to wait 3-5 seconds
	// to wait Controller broadcast, maybe this Node has register
	// in other Node, but the Node register response not broadcast
	// to this Node, so we try to wait Controller's broadcast.
	timer := time.NewTicker(50 * time.Millisecond)
	defer timer.Stop()
	times := 60 + server.rand.Int(40)
	for i := 0; i < times; i++ {
		nk = server.ctx.storage.GetNodeKey(guid)
		if nk != nil {
			return nk
		}
		select {
		case <-server.context.Done():
			return nil
		case <-timer.C:
		}
	}
	// If still doesn't exist, it means this Node register just now,
	// so this Node can't receive the last broadcast.
	// This Node will try to query Node key from controller, but
	// Controller maybe not connect the Node Network, so it does't
	// guarantee that the Node key can be queried.
	now := server.ctx.global.Now()
	query := messages.QueryNodeKey{
		GUID: *guid,
		Time: now,
	}
	// <security> must don't handle error.
	_ = server.ctx.sender.Send(server.context,
		messages.CMDBQueryNodeKey, query, true)
	// calculate network latency between Node and Controller.
	latency := server.ctx.global.Now().Sub(now)
	// wait Controller send Node key to this Node.
	times = int(latency/(50*time.Millisecond)) + 60 + server.rand.Int(40)
	for i := 0; i < times; i++ {
		nk = server.ctx.storage.GetNodeKey(guid)
		if nk != nil {
			return nk
		}
		select {
		case <-server.context.Done():
			return nil
		case <-timer.C:
		}
	}
	return nil
}

func (server *server) verifyNode(conn *xnet.Conn, guid *guid.GUID) bool {
	challenge := server.rand.Bytes(protocol.ChallengeSize)
	err := conn.Send(challenge)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send challenge to node:", err)
		return false
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		server.logConn(conn, logger.Error, "failed to receive node signature:", err)
		return false
	}
	nk := server.getNodeKey(guid)
	if nk == nil {
		return false
	}
	// verify signature
	if !ed25519.Verify(nk.PublicKey, challenge, signature) {
		server.logConn(conn, logger.Exploit, "invalid node challenge signature")
		return false
	}
	// send succeed response
	_ = conn.SetWriteDeadline(time.Now().Add(server.timeout))
	err = conn.Send(protocol.AuthSucceed)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send response to node:", err)
		return false
	}
	return true
}

func (server *server) handshakeWithBeacon(tag *guid.GUID, conn *xnet.Conn) {
	beaconGUID := guid.GUID{}
	_, err := io.ReadFull(conn, beaconGUID[:])
	if err != nil {
		server.logConn(conn, logger.Error, "failed to receive beacon guid:", err)
		return
	}
	// read operation
	operation := make([]byte, 1)
	_, err = io.ReadFull(conn, operation)
	if err != nil {
		server.logConn(conn, logger.Exploit, "failed to receive beacon operation", err)
		return
	}
	switch operation[0] {
	case protocol.BeaconOperationRegister: // register
		server.registerBeacon(conn, &beaconGUID)
	case protocol.BeaconOperationConnect: // connect
		if !server.verifyBeacon(conn, &beaconGUID) {
			return
		}
		server.serveBeacon(tag, &beaconGUID, conn)
	default:
		server.logfConn(conn, logger.Exploit, "unknown beacon operation %d", operation[0])
	}
}

func (server *server) registerBeacon(conn *xnet.Conn, guid *guid.GUID) {
	// send external address
	err := conn.Send(nettool.EncodeExternalAddress(conn.RemoteAddr().String()))
	if err != nil {
		const log = "failed to send beacon external address:"
		server.logConn(conn, logger.Error, log, err)
		return
	}
	// Beacon send self key exchange public key (curve25519),
	// use session key encrypt register request data.
	//
	// +----------------+----------------+
	// | kex public key | encrypted data |
	// +----------------+----------------+
	// |    32 Bytes    |       var      |
	// +----------------+----------------+
	//
	// receive encrypted Beacon register request
	request, err := conn.Receive()
	if err != nil {
		const log = "failed to receive beacon register request:"
		server.logConn(conn, logger.Error, log, err)
		return
	}
	// check size
	if len(request) < curve25519.ScalarSize+aes.BlockSize {
		const log = "receive invalid encrypted beacon register request"
		server.logConn(conn, logger.Exploit, log)
		return
	}
	// create Beacon register
	response := server.ctx.storage.CreateBeaconRegister(guid)
	if response == nil {
		const format = "failed to create beacon register\nGUID: %X"
		server.logfConn(conn, logger.Warning, format, guid)
		return
	}
	// <security> must don't handle error
	_ = server.ctx.sender.Send(server.context,
		messages.CMDBBeaconRegisterRequest, request, true)
	// wait register result
	timeout := time.Duration(15+server.rand.Int(30)) * time.Second
	timer := time.AfterFunc(timeout, func() {
		defer func() {
			if r := recover(); r != nil {
				server.log(logger.Fatal, xpanic.Print(r, "server.registerNode"))
			}
		}()
		server.ctx.storage.SetBeaconRegister(guid, &messages.BeaconRegisterResponse{
			Result: messages.RegisterResultTimeout,
		})
	})
	defer timer.Stop()
	// read register response result
	var resp *messages.BeaconRegisterResponse
	select {
	case resp = <-response:
	case <-server.context.Done():
		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(server.timeout))
	switch resp.Result {
	case messages.RegisterResultAccept:
		_, _ = conn.Write([]byte{messages.RegisterResultAccept})
		_ = conn.Send(resp.NodeListeners)
	case messages.RegisterResultRefused:
		// TODO add IP black list only register(other role still pass)
		// and <firewall> rate limit

		_, _ = conn.Write([]byte{messages.RegisterResultTimeout})
	case messages.RegisterResultTimeout:
		_, _ = conn.Write([]byte{messages.RegisterResultTimeout})
	default:
		const format = "unknown beacon register result: %d"
		server.logfConn(conn, logger.Exploit, format, resp.Result)
	}
}

func (server *server) getBeaconKey(guid *guid.GUID) *protocol.BeaconKey {
	// First try to query from self storage.
	bk := server.ctx.storage.GetBeaconKey(guid)
	if bk != nil {
		return bk
	}
	// If doesn't exists in self storage, try to wait 3-5 seconds
	// to wait Controller broadcast, maybe this Beacon has register
	// in other Node, but the Beacon register response not broadcast
	// to this Node, so we try to wait Controller's broadcast.
	timer := time.NewTicker(50 * time.Millisecond)
	defer timer.Stop()
	times := 60 + server.rand.Int(40)
	for i := 0; i < times; i++ {
		bk = server.ctx.storage.GetBeaconKey(guid)
		if bk != nil {
			return bk
		}
		select {
		case <-server.context.Done():
			return nil
		case <-timer.C:
		}
	}
	// If still doesn't exist, it means this Node register just now,
	// so this Node can't receive the last broadcast.
	// This Node will try to query Beacon key from controller, but
	// Controller maybe not connect the Node Network, so it does't
	// guarantee that Beacon key can be queried.
	now := server.ctx.global.Now()
	query := messages.QueryBeaconKey{
		GUID: *guid,
		Time: now,
	}
	// <security> must don't handle error
	_ = server.ctx.sender.Send(server.context,
		messages.CMDBQueryBeaconKey, query, true)
	// calculate network latency between Node and Controller
	latency := server.ctx.global.Now().Sub(now)
	// wait Controller send Beacon key to this Node
	times = int(latency/(50*time.Millisecond)) + 60 + server.rand.Int(40)
	for i := 0; i < times; i++ {
		bk = server.ctx.storage.GetBeaconKey(guid)
		if bk != nil {
			return bk
		}
		select {
		case <-server.context.Done():
			return nil
		case <-timer.C:
		}
	}
	return nil
}

func (server *server) verifyBeacon(conn *xnet.Conn, guid *guid.GUID) bool {
	challenge := server.rand.Bytes(protocol.ChallengeSize)
	err := conn.Send(challenge)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send challenge to beacon:", err)
		return false
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		server.logConn(conn, logger.Error, "failed to receive beacon signature:", err)
		return false
	}
	bk := server.getBeaconKey(guid)
	if bk == nil {
		return false
	}
	// verify signature
	if !ed25519.Verify(bk.PublicKey, challenge, signature) {
		server.logConn(conn, logger.Exploit, "invalid beacon challenge signature")
		return false
	}
	// send succeed response
	_ = conn.SetWriteDeadline(time.Now().Add(server.timeout))
	err = conn.Send(protocol.AuthSucceed)
	if err != nil {
		server.logConn(conn, logger.Error, "failed to send response to beacon:", err)
		return false
	}
	return true
}

// ---------------------------------------serve controller-----------------------------------------

type ctrlConn struct {
	ctx *Node

	Tag  *guid.GUID
	Conn *conn

	inSync int32
	syncM  sync.Mutex
}

func (server *server) serveCtrl(tag *guid.GUID, conn *xnet.Conn) {
	cc := ctrlConn{
		ctx:  server.ctx,
		Tag:  tag,
		Conn: newConn(server.ctx, conn, tag, connUsageServeCtrl),
	}
	defer func() {
		if r := recover(); r != nil {
			cc.Conn.Log(logger.Fatal, xpanic.Print(r, "server.serveCtrl"))
		}
		// logoff forwarder
		cc.syncM.Lock()
		defer cc.syncM.Unlock()
		if cc.isSync() {
			server.ctx.forwarder.LogoffCtrl(tag)
		}
		cc.Close()
		cc.Conn.Log(logger.Debug, "disconnected")
	}()
	server.addCtrlConn(tag, &cc)
	defer server.deleteCtrlConn(tag)
	_ = conn.SetDeadline(time.Time{})
	cc.Conn.Log(logger.Debug, "connected")
	protocol.HandleConn(conn, cc.onFrame)
}

func (ctrl *ctrlConn) isSync() bool {
	return atomic.LoadInt32(&ctrl.inSync) != 0
}

func (ctrl *ctrlConn) onFrame(frame []byte) {
	if ctrl.Conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnSendHeartbeat {
		ctrl.Conn.HandleHeartbeat()
		return
	}
	if ctrl.isSync() {
		if ctrl.onFrameAfterSync(frame) {
			return
		}
	} else {
		if ctrl.onFrameBeforeSync(frame) {
			return
		}
	}
	const format = "unknown command: %d\nframe:\n%s"
	ctrl.Conn.Logf(logger.Exploit, format, frame[0], spew.Sdump(frame))
	ctrl.Close()
}

func (ctrl *ctrlConn) onFrameBeforeSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	switch frame[0] {
	case protocol.CtrlSync:
		ctrl.handleSyncStart(id)
	case protocol.CtrlTrustNode:
		ctrl.handleTrustNode(id)
	case protocol.CtrlSetNodeCert:
		ctrl.handleSetCertificate(id, data)
	case protocol.CtrlQueryKeyStorage:
		ctrl.handleQueryKeyStorage(id)
	default:
		return false
	}
	return true
}

func (ctrl *ctrlConn) handleSyncStart(id []byte) {
	ctrl.syncM.Lock()
	defer ctrl.syncM.Unlock()
	if ctrl.isSync() {
		return
	}
	// initialize sync pool
	ctrl.Conn.SendPool.New = func() interface{} {
		return protocol.NewSend()
	}
	ctrl.Conn.AckPool.New = func() interface{} {
		return protocol.NewAcknowledge()
	}
	ctrl.Conn.AnswerPool.New = func() interface{} {
		return protocol.NewAnswer()
	}
	// must presume, or may be lost message.
	atomic.StoreInt32(&ctrl.inSync, 1)
	err := ctrl.ctx.forwarder.RegisterCtrl(ctrl)
	if err != nil {
		atomic.StoreInt32(&ctrl.inSync, 0)
		ctrl.Conn.Reply(id, []byte(err.Error()))
		ctrl.Close()
		return
	}
	ctrl.Conn.Reply(id, []byte{protocol.NodeSync})
	ctrl.Conn.Log(logger.Debug, "start to synchronize")
}

func (ctrl *ctrlConn) onFrameAfterSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	switch frame[0] {
	case protocol.CtrlSendToNodeGUID:
		ctrl.Conn.HandleSendToNodeGUID(id, data)
	case protocol.CtrlSendToNode:
		ctrl.Conn.HandleSendToNode(id, data)
	case protocol.CtrlAckToNodeGUID:
		ctrl.Conn.HandleAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		ctrl.Conn.HandleAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		ctrl.Conn.HandleSendToBeaconGUID(id, data)
	case protocol.CtrlSendToBeacon:
		ctrl.Conn.HandleSendToBeacon(id, data)
	case protocol.CtrlAckToBeaconGUID:
		ctrl.Conn.HandleAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		ctrl.Conn.HandleAckToBeacon(id, data)
	case protocol.CtrlBroadcastGUID:
		ctrl.Conn.HandleBroadcastGUID(id, data)
	case protocol.CtrlBroadcast:
		ctrl.Conn.HandleBroadcast(id, data)
	case protocol.CtrlAnswerGUID:
		ctrl.Conn.HandleAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		ctrl.Conn.HandleAnswer(id, data)
	default:
		return false
	}
	return true
}

func (ctrl *ctrlConn) handleTrustNode(id []byte) {
	ctrl.Conn.Reply(id, ctrl.ctx.register.PackRequest("trust"))
}

func (ctrl *ctrlConn) handleSetCertificate(id []byte, data []byte) {
	err := ctrl.ctx.global.SetCertificate(data)
	if err == nil {
		ctrl.Conn.Reply(id, []byte{messages.RegisterResultAccept})
		ctrl.Conn.Log(logger.Debug, "trust node")
	} else {
		ctrl.Conn.Reply(id, []byte(err.Error()))
	}
}

func (ctrl *ctrlConn) handleQueryKeyStorage(id []byte) {
	ks := protocol.KeyStorage{
		NodeKeys:   ctrl.ctx.storage.GetAllNodeKeys(),
		BeaconKeys: ctrl.ctx.storage.GetAllBeaconKeys(),
	}
	data, err := msgpack.Marshal(ks)
	if err != nil {
		ctrl.Conn.Reply(id, []byte(err.Error()))
	} else {
		ctrl.Conn.Reply(id, data)
	}
}

func (ctrl *ctrlConn) Close() {
	_ = ctrl.Conn.Close()
}

// ------------------------------------------serve node--------------------------------------------

type nodeConn struct {
	ctx *Node

	GUID *guid.GUID
	Conn *conn

	inSync int32
	syncM  sync.Mutex
}

func (server *server) serveNode(tag, nodeGUID *guid.GUID, conn *xnet.Conn) {
	nc := nodeConn{
		ctx:  server.ctx,
		GUID: nodeGUID,
		Conn: newConn(server.ctx, conn, nodeGUID, connUsageServeNode),
	}
	defer func() {
		if r := recover(); r != nil {
			nc.Conn.Log(logger.Fatal, xpanic.Print(r, "server.serveNode"))
		}
		// logoff forwarder
		nc.syncM.Lock()
		defer nc.syncM.Unlock()
		if nc.isSync() {
			server.ctx.forwarder.LogoffNode(nodeGUID)
		}
		nc.Close()
		nc.Conn.Log(logger.Debug, "disconnected")
	}()
	server.addNodeConn(tag, &nc)
	defer server.deleteNodeConn(tag)
	_ = conn.SetDeadline(time.Time{})
	nc.Conn.Log(logger.Debug, "connected")
	protocol.HandleConn(conn, nc.onFrame)
}

func (node *nodeConn) isSync() bool {
	return atomic.LoadInt32(&node.inSync) != 0
}

func (node *nodeConn) onFrame(frame []byte) {
	if node.Conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnSendHeartbeat {
		node.Conn.HandleHeartbeat()
		return
	}
	if node.isSync() {
		if node.onFrameAfterSync(frame) {
			return
		}
	} else {
		if node.onFrameBeforeSync(frame) {
			return
		}
	}
	const format = "unknown command: %d\nframe:\n%s"
	node.Conn.Logf(logger.Exploit, format, frame[0], spew.Sdump(frame))
	node.Close()
}

func (node *nodeConn) onFrameBeforeSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	switch frame[0] {
	case protocol.NodeSync:
		node.handleSyncStart(id)
	default:
		return false
	}
	return true
}

func (node *nodeConn) handleSyncStart(id []byte) {
	node.syncM.Lock()
	defer node.syncM.Unlock()
	if node.isSync() {
		return
	}
	// initialize sync pool
	node.Conn.SendPool.New = func() interface{} {
		return protocol.NewSend()
	}
	node.Conn.AckPool.New = func() interface{} {
		return protocol.NewAcknowledge()
	}
	node.Conn.AnswerPool.New = func() interface{} {
		return protocol.NewAnswer()
	}
	node.Conn.QueryPool.New = func() interface{} {
		return protocol.NewQuery()
	}
	// must presume, or may be lost message.
	atomic.StoreInt32(&node.inSync, 1)
	err := node.ctx.forwarder.RegisterNode(node)
	if err != nil {
		atomic.StoreInt32(&node.inSync, 0)
		node.Conn.Reply(id, []byte(err.Error()))
		node.Close()
		return
	}
	node.Conn.Reply(id, []byte{protocol.NodeSync})
	node.Conn.Log(logger.Info, "start to synchronize")
}

func (node *nodeConn) onFrameAfterSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	if node.onFrameAfterSyncAboutCtrl(frame[0], id, data) {
		return true
	}
	if node.onFrameAfterSyncAboutNode(frame[0], id, data) {
		return true
	}
	if node.onFrameAfterSyncAboutBeacon(frame[0], id, data) {
		return true
	}
	return false
}

func (node *nodeConn) onFrameAfterSyncAboutCtrl(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.CtrlSendToNodeGUID:
		node.Conn.HandleSendToNodeGUID(id, data)
	case protocol.CtrlSendToNode:
		node.Conn.HandleSendToNode(id, data)
	case protocol.CtrlAckToNodeGUID:
		node.Conn.HandleAckToNodeGUID(id, data)
	case protocol.CtrlAckToNode:
		node.Conn.HandleAckToNode(id, data)
	case protocol.CtrlSendToBeaconGUID:
		node.Conn.HandleSendToBeaconGUID(id, data)
	case protocol.CtrlSendToBeacon:
		node.Conn.HandleSendToBeacon(id, data)
	case protocol.CtrlAckToBeaconGUID:
		node.Conn.HandleAckToBeaconGUID(id, data)
	case protocol.CtrlAckToBeacon:
		node.Conn.HandleAckToBeacon(id, data)
	case protocol.CtrlBroadcastGUID:
		node.Conn.HandleBroadcastGUID(id, data)
	case protocol.CtrlBroadcast:
		node.Conn.HandleBroadcast(id, data)
	case protocol.CtrlAnswerGUID:
		node.Conn.HandleAnswerGUID(id, data)
	case protocol.CtrlAnswer:
		node.Conn.HandleAnswer(id, data)
	default:
		return false
	}
	return true
}

func (node *nodeConn) onFrameAfterSyncAboutNode(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.NodeSendGUID:
		node.Conn.HandleNodeSendGUID(id, data)
	case protocol.NodeSend:
		node.Conn.HandleNodeSend(id, data)
	case protocol.NodeAckGUID:
		node.Conn.HandleNodeAckGUID(id, data)
	case protocol.NodeAck:
		node.Conn.HandleNodeAck(id, data)
	default:
		return false
	}
	return true
}

func (node *nodeConn) onFrameAfterSyncAboutBeacon(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.BeaconSendGUID:
		node.Conn.HandleBeaconSendGUID(id, data)
	case protocol.BeaconSend:
		node.Conn.HandleBeaconSend(id, data)
	case protocol.BeaconAckGUID:
		node.Conn.HandleBeaconAckGUID(id, data)
	case protocol.BeaconAck:
		node.Conn.HandleBeaconAck(id, data)
	case protocol.BeaconQueryGUID:
		node.Conn.HandleQueryGUID(id, data)
	case protocol.BeaconQuery:
		node.Conn.HandleQuery(id, data)
	default:
		return false
	}
	return true
}

func (node *nodeConn) Close() {
	_ = node.Conn.Close()
}

// -----------------------------------------serve beacon-------------------------------------------

type beaconConn struct {
	ctx *Node

	GUID *guid.GUID // beacon guid
	Conn *conn

	inSync int32
	syncM  sync.Mutex
}

func (server *server) serveBeacon(tag, beaconGUID *guid.GUID, conn *xnet.Conn) {
	bc := beaconConn{
		ctx:  server.ctx,
		GUID: beaconGUID,
		Conn: newConn(server.ctx, conn, beaconGUID, connUsageServeBeacon),
	}
	defer func() {
		if r := recover(); r != nil {
			bc.Conn.Log(logger.Fatal, xpanic.Print(r, "server.serveBeacon"))
		}
		// logoff forwarder
		bc.syncM.Lock()
		defer bc.syncM.Unlock()
		if bc.isSync() {
			server.ctx.forwarder.LogoffBeacon(beaconGUID)
		}
		bc.Close()
		bc.Conn.Log(logger.Debug, "disconnected")
	}()
	server.addBeaconConn(tag, &bc)
	defer server.deleteBeaconConn(tag)
	_ = conn.SetDeadline(time.Time{})
	bc.Conn.Log(logger.Debug, "connected")
	protocol.HandleConn(conn, bc.onFrame)
}

func (beacon *beaconConn) isSync() bool {
	return atomic.LoadInt32(&beacon.inSync) != 0
}

func (beacon *beaconConn) onFrame(frame []byte) {
	if beacon.Conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnSendHeartbeat {
		beacon.Conn.HandleHeartbeat()
		return
	}
	if beacon.isSync() {
		if beacon.onFrameAfterSync(frame) {
			return
		}
	} else {
		if beacon.onFrameBeforeSync(frame) {
			return
		}
	}
	const format = "unknown command: %d\nframe:\n%s"
	beacon.Conn.Logf(logger.Exploit, format, frame[0], spew.Sdump(frame))
	beacon.Close()
}

func (beacon *beaconConn) onFrameBeforeSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	switch frame[0] {
	case protocol.BeaconSync:
		beacon.handleSyncStart(id)
	default:
		return false
	}
	return true
}

func (beacon *beaconConn) handleSyncStart(id []byte) {
	beacon.syncM.Lock()
	defer beacon.syncM.Unlock()
	if beacon.isSync() {
		return
	}
	// initialize sync pool
	beacon.Conn.SendPool.New = func() interface{} {
		return protocol.NewSend()
	}
	beacon.Conn.AckPool.New = func() interface{} {
		return protocol.NewAcknowledge()
	}
	beacon.Conn.QueryPool.New = func() interface{} {
		return protocol.NewQuery()
	}
	// must presume, or may be lost message.
	atomic.StoreInt32(&beacon.inSync, 1)
	err := beacon.ctx.forwarder.RegisterBeacon(beacon)
	if err != nil {
		atomic.StoreInt32(&beacon.inSync, 0)
		beacon.Conn.Reply(id, []byte(err.Error()))
		beacon.Close()
		return
	}
	beacon.Conn.Reply(id, []byte{protocol.NodeSync})
	beacon.Conn.Log(logger.Info, "start to synchronize")
}

func (beacon *beaconConn) onFrameAfterSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	switch frame[0] {
	case protocol.BeaconSendGUID:
		beacon.Conn.HandleBeaconSendGUID(id, data)
	case protocol.BeaconSend:
		beacon.Conn.HandleBeaconSend(id, data)
	case protocol.BeaconAckGUID:
		beacon.Conn.HandleBeaconAckGUID(id, data)
	case protocol.BeaconAck:
		beacon.Conn.HandleBeaconAck(id, data)
	case protocol.BeaconQueryGUID:
		beacon.Conn.HandleQueryGUID(id, data)
	case protocol.BeaconQuery:
		beacon.Conn.HandleQuery(id, data)
	default:
		return false
	}
	return true
}

func (beacon *beaconConn) Close() {
	_ = beacon.Conn.Close()
}

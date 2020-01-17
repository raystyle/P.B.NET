package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"
	"golang.org/x/net/netutil"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xnet"
	"project/internal/xpanic"
)

var (
	errServerClosed = fmt.Errorf("server closed")
)

// accept beacon node controller
type server struct {
	ctx *Node

	maxConns int           // every listener
	timeout  time.Duration // handshake timeout

	guid         *guid.Generator
	rand         *random.Rand
	listeners    map[string]*Listener // key = tag
	listenersRWM sync.RWMutex
	conns        map[string]*xnet.Conn
	connsRWM     sync.RWMutex

	ctrlConns      map[string]*ctrlConn
	ctrlConnsRWM   sync.RWMutex
	nodeConns      map[string]*nodeConn
	nodeConnsRWM   sync.RWMutex
	beaconConns    map[string]*beaconConn
	beaconConnsRWM sync.RWMutex

	inShutdown int32
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// Listener is used to wrap net.Listener with xnet.Mode
type Listener struct {
	Mode string
	net.Listener
}

func newServer(ctx *Node, config *Config) (*server, error) {
	cfg := config.Server

	if cfg.MaxConns < 1 {
		return nil, errors.New("listener max connection must > 0")
	}
	if cfg.Timeout < 15*time.Second {
		return nil, errors.New("listener max connection must >= 15s")
	}

	memory := security.NewMemory()
	defer memory.Flush()

	server := server{
		ctx:         ctx,
		maxConns:    cfg.MaxConns,
		timeout:     cfg.Timeout,
		rand:        random.New(),
		listeners:   make(map[string]*Listener),
		conns:       make(map[string]*xnet.Conn),
		ctrlConns:   make(map[string]*ctrlConn),
		nodeConns:   make(map[string]*nodeConn),
		beaconConns: make(map[string]*beaconConn),
		stopSignal:  make(chan struct{}),
	}
	server.guid = guid.New(1024, server.ctx.global.Now)

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
func (s *server) Deploy() error {
	// deploy all listener
	l := len(s.listeners)
	errs := make(chan error, l)
	for tag, listener := range s.listeners {
		go func(tag string, listener *Listener) {
			errs <- s.deploy(tag, listener)
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

func (s *server) logf(lv logger.Level, format string, log ...interface{}) {
	s.ctx.logger.Printf(lv, "server", format, log...)
}

func (s *server) log(lv logger.Level, log ...interface{}) {
	s.ctx.logger.Println(lv, "server", log...)
}

func (s *server) addListener(l *messages.Listener) (*Listener, error) {
	tlsConfig, err := l.TLSConfig.Apply()
	if err != nil {
		return nil, errors.Wrapf(err, "listener %s", l.Tag)
	}
	tlsConfig.Time = s.ctx.global.Now // <security>
	cfg := xnet.Config{
		Network:   l.Network,
		Address:   l.Address,
		Timeout:   l.Timeout,
		TLSConfig: tlsConfig,
	}
	listener, err := xnet.Listen(l.Mode, &cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen %s", l.Tag)
	}
	listener = netutil.LimitListener(listener, s.maxConns)
	ll := &Listener{Mode: l.Mode, Listener: listener}
	s.listenersRWM.Lock()
	defer s.listenersRWM.Unlock()
	if _, ok := s.listeners[l.Tag]; !ok {
		s.listeners[l.Tag] = ll
		return ll, nil
	}
	return nil, errors.Errorf("listener %s already exists", l.Tag)
}

func (s *server) deploy(tag string, listener *Listener) error {
	errChan := make(chan error, 1)
	s.wg.Add(1)
	go s.serve(tag, listener, errChan)
	select {
	case err := <-errChan:
		const format = "failed to deploy listener %s(%s): %s"
		return errors.Errorf(format, tag, listener.Addr(), err)
	case <-time.After(time.Second):
		network := listener.Addr().Network()
		address := listener.Addr().String()
		s.logf(logger.Info, "deploy listener %s: %s %s", tag, network, address)
		return nil
	}
}

func (s *server) serve(tag string, l *Listener, errChan chan<- error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "server.serve()")
			s.log(logger.Fatal, err)
		}
		errChan <- err
		close(errChan)
		// delete
		s.listenersRWM.Lock()
		defer s.listenersRWM.Unlock()
		delete(s.listeners, tag)
		s.logf(logger.Info, "listener: %s (%s) is closed", tag, l.Addr())
		s.wg.Done()
	}()
	var delay time.Duration // how long to sleep on accept failure
	maxDelay := 2 * time.Second
	for {
		conn, e := l.Accept()
		if e != nil {
			select {
			case <-s.stopSignal:
				err = errors.WithStack(errServerClosed)
				return
			default:
			}
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if delay > maxDelay {
					delay = maxDelay
				}
				s.logf(logger.Warning, "accept error: %s; retrying in %v", e, delay)
				time.Sleep(delay)
				continue
			}
			if !strings.Contains(e.Error(), "use of closed network connection") {
				err = e
			}
			return
		}
		delay = 0
		s.wg.Add(1)
		go s.handshake(tag, conn)
	}
}

func (s *server) shuttingDown() bool {
	return atomic.LoadInt32(&s.inShutdown) != 0
}

func (s *server) AddListener(l *messages.Listener) error {
	if s.shuttingDown() {
		return errors.WithStack(errServerClosed)
	}
	listener, err := s.addListener(l)
	if err != nil {
		return err
	}
	return s.deploy(l.Tag, listener)
}

func (s *server) Listeners() map[string]*Listener {
	s.listenersRWM.RLock()
	defer s.listenersRWM.RUnlock()
	listeners := make(map[string]*Listener, len(s.listeners))
	for tag, listener := range s.listeners {
		listeners[tag] = listener
	}
	return listeners
}

func (s *server) GetListener(tag string) (*Listener, error) {
	s.listenersRWM.RLock()
	defer s.listenersRWM.RUnlock()
	if listener, ok := s.listeners[tag]; ok {
		return listener, nil
	}
	return nil, errors.Errorf("listener %s doesn't exists", tag)
}

func (s *server) Conns() map[string]*xnet.Conn {
	s.connsRWM.RLock()
	defer s.connsRWM.RUnlock()
	conns := make(map[string]*xnet.Conn, len(s.conns))
	for tag, conn := range s.conns {
		conns[tag] = conn
	}
	return conns
}

// func (s *server) CloseListener(tag string) {
//
// }
//
// func (s *server) CloseConn(address string) {
//
// }

// Close is used to close all listeners and connections
func (s *server) Close() {
	atomic.StoreInt32(&s.inShutdown, 1)
	close(s.stopSignal)
	// close all listeners
	for _, listener := range s.Listeners() {
		_ = listener.Close()
	}
	// close all conns
	for _, conn := range s.Conns() {
		_ = conn.Close()
	}
	s.guid.Close()
	s.wg.Wait()
	s.ctx = nil
}

func (s *server) logfConn(c *xnet.Conn, lv logger.Level, format string, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, format, log...)
	_, _ = fmt.Fprintf(b, "\n%s", c)
	s.ctx.logger.Print(lv, "server", b)
}

func (s *server) logConn(c *xnet.Conn, lv logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintln(b, log...)
	_, _ = fmt.Fprintf(b, "%s", c)
	s.ctx.logger.Print(lv, "server", b)
}

func (s *server) addConn(tag string, conn *xnet.Conn) {
	s.connsRWM.Lock()
	defer s.connsRWM.Unlock()
	s.conns[tag] = conn
}

func (s *server) deleteConn(tag string) {
	s.connsRWM.Lock()
	defer s.connsRWM.Unlock()
	delete(s.conns, tag)
}

func (s *server) addCtrlConn(tag string, conn *ctrlConn) {
	s.ctrlConnsRWM.Lock()
	defer s.ctrlConnsRWM.Unlock()
	if _, ok := s.ctrlConns[tag]; !ok {
		s.ctrlConns[tag] = conn
	}
}

func (s *server) deleteCtrlConn(tag string) {
	s.ctrlConnsRWM.Lock()
	defer s.ctrlConnsRWM.Unlock()
	delete(s.ctrlConns, tag)
}

func (s *server) addNodeConn(tag string, conn *nodeConn) {
	s.nodeConnsRWM.Lock()
	defer s.nodeConnsRWM.Unlock()
	if _, ok := s.nodeConns[tag]; !ok {
		s.nodeConns[tag] = conn
	}
}

func (s *server) deleteNodeConn(tag string) {
	s.nodeConnsRWM.Lock()
	defer s.nodeConnsRWM.Unlock()
	delete(s.nodeConns, tag)
}

func (s *server) addBeaconConn(tag string, conn *beaconConn) {
	s.beaconConnsRWM.Lock()
	defer s.beaconConnsRWM.Unlock()
	if _, ok := s.beaconConns[tag]; !ok {
		s.beaconConns[tag] = conn
	}
}

func (s *server) deleteBeaconConn(tag string) {
	s.beaconConnsRWM.Lock()
	defer s.beaconConnsRWM.Unlock()
	delete(s.beaconConns, tag)
}

func (s *server) handshake(tag string, conn net.Conn) {
	now := s.ctx.global.Now()
	xConn := xnet.NewConn(conn, now)
	defer func() {
		if r := recover(); r != nil {
			s.logConn(xConn, logger.Exploit, xpanic.Print(r, "server.handshake"))
		}
		_ = xConn.Close()
		s.wg.Done()
	}()
	// add to server.conns for management
	connTag := tag + hex.EncodeToString(s.guid.Get())
	s.addConn(connTag, xConn)
	defer s.deleteConn(connTag)
	_ = xConn.SetDeadline(now.Add(s.timeout))
	if !s.checkConn(xConn) {
		return
	}
	if !s.sendCertificate(xConn) {
		return
	}
	// receive role
	r := make([]byte, 1)
	_, err := io.ReadFull(xConn, r)
	if err != nil {
		s.logConn(xConn, logger.Error, "failed to receive role")
		return
	}
	role := protocol.Role(r[0])
	switch role {
	case protocol.Ctrl:
		s.handleCtrl(connTag, xConn)
	case protocol.Node:
		s.handleNode(connTag, xConn)
	case protocol.Beacon:
		s.handleBeacon(connTag, xConn)
	default:
		s.logConn(xConn, logger.Exploit, role)
	}
}

// checkConn is used to check connection is from client
func (s *server) checkConn(conn *xnet.Conn) bool {
	size := byte(100 + s.rand.Int(156))
	data := s.rand.Bytes(int(size))
	_, err := conn.Write(append([]byte{size}, data...))
	if err != nil {
		s.logConn(conn, logger.Error, "failed to send check connection data:", err)
		return false
	}
	n, err := io.ReadFull(conn, data)
	if err != nil {
		d := data[:n]
		s.logfConn(conn, logger.Error, "receive test data in checkConn\n%s\n\n%X", d, d)
		return false
	}
	return true
}

func (s *server) sendCertificate(conn *xnet.Conn) bool {
	var err error
	cert := s.ctx.global.GetCertificate()
	if cert != nil {
		err = conn.Send(cert)
		if err != nil {
			s.logConn(conn, logger.Error, "failed to send certificate:", err)
			return false
		}
	} else { // if no certificate send padding data
		size := 1024 + s.rand.Int(1024)
		err = conn.Send(s.rand.Bytes(size))
		if err != nil {
			s.logConn(conn, logger.Error, "failed to send padding data:", err)
			return false
		}
	}
	return true
}

func (s *server) handleCtrl(tag string, conn *xnet.Conn) {
	// <danger>
	// send random challenge code(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and controller sign it
	challenge := s.rand.Bytes(2048 + s.rand.Int(2048))
	err := conn.Send(challenge)
	if err != nil {
		s.logConn(conn, logger.Error, "failed to send challenge to controller:", err)
		return
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		s.logConn(conn, logger.Error, "failed to receive controller signature:", err)
		return
	}
	// verify signature
	if !s.ctx.global.CtrlVerify(challenge, signature) {
		s.logConn(conn, logger.Exploit, "invalid controller signature")
		return
	}
	// send succeed response
	err = conn.Send(protocol.AuthSucceed)
	if err != nil {
		s.logConn(conn, logger.Error, "failed to send response to controller:", err)
		return
	}
	s.serveCtrl(tag, conn)
}

const (
	nodeOperationRegister byte = iota + 1
	nodeOperationConnect
)

func (s *server) handleNode(tag string, conn *xnet.Conn) {
	nodeGUID := make([]byte, guid.Size)
	_, err := io.ReadFull(conn, nodeGUID)
	if err != nil {
		s.logConn(conn, logger.Error, "failed to receive node guid:", err)
		return
	}
	// check is self
	if bytes.Equal(nodeGUID, s.ctx.global.GUID()) {
		s.logConn(conn, logger.Debug, "oh! self")
		return
	}
	// read operation
	operation := make([]byte, 1)
	_, err = io.ReadFull(conn, operation)
	if err != nil {
		s.logConn(conn, logger.Exploit, "failed to receive node operation", err)
		return
	}
	switch operation[0] {
	case nodeOperationRegister: // register
		s.registerNode(conn, nodeGUID)
	case nodeOperationConnect: // connect
		if !s.verifyNode(conn, nodeGUID) {
			return
		}
		s.serveNode(tag, nodeGUID, conn)
	default:
		s.logfConn(conn, logger.Exploit, "unknown node operation %d", operation[0])
	}
}

func (s *server) registerNode(conn *xnet.Conn, guid []byte) {
	// receive node register request
	req, err := conn.Receive()
	if err != nil {
		s.logConn(conn, logger.Error, "failed to receive node register request:", err)
		return
	}
	// try to unmarshal
	nrr := new(messages.NodeRegisterRequest)
	err = msgpack.Unmarshal(req, nrr)
	if err != nil {
		s.logConn(conn, logger.Exploit, "invalid node register request data:", err)
		return
	}
	err = nrr.Validate()
	if err != nil {
		s.logConn(conn, logger.Exploit, "invalid node register request:", err)
		return
	}
	// create node register
	response := s.ctx.storage.CreateNodeRegister(guid)
	if response == nil {
		_ = conn.Send([]byte{messages.RegisterResultRefused})
		s.logfConn(conn, logger.Exploit, "failed to create node register\nguid: %X", guid)
		return
	}
	// send node register request to controller
	// <security> must don't handle error
	_ = s.ctx.sender.Send(messages.CMDBNodeRegisterRequest, nrr)
	// wait register result
	timeout := time.Duration(15+s.rand.Int(30)) * time.Second
	timer := time.AfterFunc(timeout, func() {
		s.ctx.storage.SetNodeRegister(guid, &messages.NodeRegisterResponse{
			Result: messages.RegisterResultTimeout,
		})
	})
	defer timer.Stop()
	resp := <-response
	switch resp.Result {
	case messages.RegisterResultAccept:
		_ = conn.Send([]byte{messages.RegisterResultAccept})
		if !s.verifyNode(conn, guid) {
			_ = conn.Send([]byte{messages.RegisterResultRefused})
			return
		}
		// send certificate and listener configs
	case messages.RegisterResultRefused: // TODO add IP black list only register(other role still pass)
		_ = conn.Send([]byte{messages.RegisterResultRefused})
	case messages.RegisterResultTimeout:
		_ = conn.Send([]byte{messages.RegisterResultTimeout})
	default:
		s.logfConn(conn, logger.Exploit, "unknown register result: %d", resp.Result)
	}
}

func (s *server) verifyNode(conn *xnet.Conn, guid []byte) bool {
	challenge := s.rand.Bytes(2048 + s.rand.Int(2048))
	err := conn.Send(challenge)
	if err != nil {
		s.logConn(conn, logger.Error, "failed to send challenge to node:", err)
		return false
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		s.logConn(conn, logger.Error, "failed to receive node signature:", err)
		return false
	}
	// verify signature
	sk := s.ctx.storage.GetNodeSessionKey(guid)
	if sk == nil {
		// TODO try to query from controller
		return false
	}
	if ed25519.Verify(sk.PublicKey, challenge, signature) {
		s.logConn(conn, logger.Exploit, "invalid node challenge signature")
		return false
	}
	// send succeed response
	err = conn.Send(protocol.AuthSucceed)
	if err != nil {
		s.logConn(conn, logger.Error, "failed to send response to node:", err)
		return false
	}
	return true
}

func (s *server) handleBeacon(tag string, conn *xnet.Conn) {
	beaconGUID, err := conn.Receive()
	if err != nil {
		s.logConn(conn, logger.Error, "failed to receive beacon guid:", err)
		return
	}
	if len(beaconGUID) != guid.Size {
		s.logConn(conn, logger.Exploit, "invalid beacon guid size")
		return
	}

	s.serveBeacon(tag, beaconGUID, conn)
}

// ---------------------------------------serve controller-----------------------------------------

type ctrlConn struct {
	ctx *Node

	tag  string
	Conn *conn

	inSync int32
	syncM  sync.Mutex
}

func (s *server) serveCtrl(tag string, conn *xnet.Conn) {
	cc := ctrlConn{
		ctx:  s.ctx,
		tag:  tag,
		Conn: newConn(s.ctx, conn, protocol.CtrlGUID, connUsageServeCtrl),
	}
	defer func() {
		if r := recover(); r != nil {
			cc.Conn.Log(logger.Exploit, xpanic.Print(r, "server.serveCtrl"))
		}
		cc.Close()
		if cc.isSync() {
			s.ctx.forwarder.LogoffCtrl(tag)
		}
		s.deleteCtrlConn(tag)
		cc.Conn.Log(logger.Debug, "controller disconnected")
	}()
	s.addCtrlConn(tag, &cc)
	cc.Conn.Log(logger.Debug, "controller connected")
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
	default:
		return false
	}
	return true
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

func (ctrl *ctrlConn) handleSyncStart(id []byte) {
	ctrl.syncM.Lock()
	defer ctrl.syncM.Unlock()
	if ctrl.isSync() {
		return
	}
	ctrl.Conn.SendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	ctrl.Conn.AckPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	ctrl.Conn.AnswerPool.New = func() interface{} {
		return &protocol.Answer{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Message:    make([]byte, aes.BlockSize),
			Hash:       make([]byte, sha256.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	err := ctrl.ctx.forwarder.RegisterCtrl(ctrl.tag, ctrl)
	if err != nil {
		ctrl.Conn.Reply(id, []byte(err.Error()))
		ctrl.Close()
	} else {
		atomic.StoreInt32(&ctrl.inSync, 1)
		ctrl.Conn.Reply(id, []byte{protocol.NodeSync})
		ctrl.Conn.Log(logger.Debug, "synchronizing")
	}
}

func (ctrl *ctrlConn) handleTrustNode(id []byte) {
	ctrl.Conn.Reply(id, ctrl.ctx.register.PackRequest())
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

func (ctrl *ctrlConn) Close() {
	_ = ctrl.Conn.Close()
}

// ------------------------------------------serve node--------------------------------------------

type nodeConn struct {
	ctx *Node

	tag  string
	guid []byte
	Conn *conn

	inSync int32
	syncM  sync.Mutex
}

func (s *server) serveNode(tag string, nodeGUID []byte, conn *xnet.Conn) {
	nc := nodeConn{
		ctx:  s.ctx,
		tag:  tag,
		guid: nodeGUID,
		Conn: newConn(s.ctx, conn, nodeGUID, connUsageServeNode),
	}
	defer func() {
		if r := recover(); r != nil {
			nc.Conn.Log(logger.Exploit, xpanic.Print(r, "server.serveNode"))
		}
		nc.Close()
		if nc.isSync() {
			s.ctx.forwarder.LogoffNode(tag)
		}
		s.deleteNodeConn(tag)
		nc.Conn.Logf(logger.Debug, "node %X disconnected", nodeGUID)
	}()
	s.addNodeConn(tag, &nc)
	_ = conn.SetDeadline(s.ctx.global.Now().Add(s.timeout))
	nc.Conn.Logf(logger.Debug, "node %X connected", nodeGUID)
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

func (node *nodeConn) onFrameAfterSync(frame []byte) bool {
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	if node.onFrameAfterSyncAboutCTRL(frame[0], id, data) {
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

func (node *nodeConn) onFrameAfterSyncAboutCTRL(cmd byte, id, data []byte) bool {
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
		node.Conn.HandleBeaconQueryGUID(id, data)
	case protocol.BeaconQuery:
		node.Conn.HandleBeaconQuery(id, data)
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
	node.Conn.SendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	node.Conn.AckPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	node.Conn.AnswerPool.New = func() interface{} {
		return &protocol.Answer{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Message:    make([]byte, aes.BlockSize),
			Hash:       make([]byte, sha256.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	node.Conn.QueryPool.New = func() interface{} {
		return &protocol.Query{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	err := node.ctx.forwarder.RegisterNode(node.tag, node)
	if err != nil {
		node.Conn.Reply(id, []byte(err.Error()))
		node.Close()
	} else {
		atomic.StoreInt32(&node.inSync, 1)
		node.Conn.Reply(id, []byte{protocol.NodeSync})
		node.Conn.Log(logger.Debug, "synchronizing")
	}
}

func (node *nodeConn) Close() {
	_ = node.Conn.Close()
}

// -----------------------------------------serve beacon-------------------------------------------

type beaconConn struct {
	ctx *Node

	tag  string
	guid []byte // beacon guid
	Conn *conn

	inSync int32
	syncM  sync.Mutex
}

func (s *server) serveBeacon(tag string, beaconGUID []byte, conn *xnet.Conn) {
	bc := beaconConn{
		ctx:  s.ctx,
		tag:  tag,
		guid: beaconGUID,
		Conn: newConn(s.ctx, conn, beaconGUID, connUsageServeBeacon),
	}
	defer func() {
		if r := recover(); r != nil {
			bc.Conn.Log(logger.Exploit, xpanic.Print(r, "server.serveNode"))
		}
		bc.Close()
		if bc.isSync() {
			s.ctx.forwarder.LogoffBeacon(tag)
		}
		s.deleteBeaconConn(tag)
		bc.Conn.Logf(logger.Debug, "beacon %X disconnected", beaconGUID)
	}()
	s.addBeaconConn(tag, &bc)
	_ = conn.SetDeadline(s.ctx.global.Now().Add(s.timeout))
	bc.Conn.Logf(logger.Debug, "beacon %X connected", beaconGUID)
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
	case protocol.NodeSync:
		beacon.handleSyncStart(id)
	default:
		return false
	}
	return true
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
		beacon.Conn.HandleBeaconQueryGUID(id, data)
	case protocol.BeaconQuery:
		beacon.Conn.HandleBeaconQuery(id, data)
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
	beacon.Conn.SendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	beacon.Conn.AckPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	beacon.Conn.QueryPool.New = func() interface{} {
		return &protocol.Query{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	err := beacon.ctx.forwarder.RegisterBeacon(beacon.tag, beacon)
	if err != nil {
		beacon.Conn.Reply(id, []byte(err.Error()))
		beacon.Close()
	} else {
		atomic.StoreInt32(&beacon.inSync, 1)
		beacon.Conn.Reply(id, []byte{protocol.NodeSync})
		beacon.Conn.Log(logger.Debug, "synchronizing")
	}
}

func (beacon *beaconConn) Close() {
	_ = beacon.Conn.Close()
}

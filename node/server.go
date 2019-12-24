package node

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	conns        map[string]*xnet.Conn // key = guid
	connsRWM     sync.RWMutex

	ctrlConns      map[string]*ctrlConn
	ctrlConnsRWM   sync.RWMutex
	nodeConns      map[string]*nodeConn
	nodeConnsRWM   sync.RWMutex
	beaconConns    map[string]*beaconConn
	beaconConnsRWM sync.RWMutex

	// calculate key
	hexPool sync.Pool

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

	server := server{listeners: make(map[string]*Listener)}
	if len(cfg.Listeners) != 0 {
		// decrypt configs about listeners
		if len(cfg.AESCrypto) != aes.Key256Bit+aes.IVSize {
			return nil, errors.New("invalid aes key size")
		}
		aesKey := cfg.AESCrypto[:aes.Key256Bit]
		aesIV := cfg.AESCrypto[aes.Key256Bit:]
		defer func() {
			security.CoverBytes(aesKey)
			security.CoverBytes(aesIV)
		}()
		data, err := aes.CBCDecrypt(cfg.Listeners, aesKey, aesIV)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// load listeners
		var listeners []*messages.Listener
		err = msgpack.Unmarshal(data, &listeners)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		for i := 0; i < len(listeners); i++ {
			_, err = server.addListener(listeners[i])
			if err != nil {
				return nil, err
			}
		}
	}

	server.ctx = ctx
	server.maxConns = cfg.MaxConns
	server.timeout = cfg.Timeout
	server.guid = guid.New(1024, server.ctx.global.Now)
	server.rand = random.New()
	server.conns = make(map[string]*xnet.Conn)
	server.ctrlConns = make(map[string]*ctrlConn)
	server.nodeConns = make(map[string]*nodeConn)
	server.beaconConns = make(map[string]*beaconConn)
	server.hexPool.New = func() interface{} {
		return make([]byte, 2*guid.Size)
	}
	server.stopSignal = make(chan struct{})
	return &server, nil
}

// Deploy is used to deploy added listener
func (s *server) Deploy() error {
	// deploy all listener
	l := len(s.listeners)
	errs := make(chan error, l)
	for tag, l := range s.listeners {
		go func(tag string, l *Listener) {
			errs <- s.deploy(tag, l)
		}(tag, l)
	}
	for i := 0; i < l; i++ {
		err := <-errs
		if err != nil {
			return err
		}
	}
	return nil
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
		return nil, errors.Errorf("failed to listen %s: %s", l.Tag, err)
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
		return nil
	}
}

func (s *server) logf(lv logger.Level, format string, log ...interface{}) {
	s.ctx.logger.Printf(lv, "server", format, log...)
}

func (s *server) log(lv logger.Level, log ...interface{}) {
	s.ctx.logger.Print(lv, "server", log...)
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
	s.connsRWM.Lock()
	defer s.connsRWM.Unlock()
	for _, conn := range s.conns {
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
	_, _ = fmt.Fprint(b, log...)
	_, _ = fmt.Fprintf(b, "\n%s", c)
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
			s.logConn(xConn, logger.Exploit, xpanic.Error(r, "server.handshake"))
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
	case protocol.Beacon:
		s.handleBeacon(connTag, xConn)
	case protocol.Node:
		s.handleNode(connTag, xConn)
	case protocol.Ctrl:
		s.handleCtrl(connTag, xConn)
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
	cert := s.ctx.global.Certificate()
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

	s.serveBeacon(tag, beaconGUID, newConn(s.ctx.logger, conn, connUsageServeBeacon))
}

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
	case 1: // register
		s.registerNode(conn, nodeGUID)
	case 2: // connect
		if !s.verifyNode(conn, nodeGUID) {
			return
		}
		s.serveNode(tag, nodeGUID, newConn(s.ctx.logger, conn, connUsageServeNode))
	default:
		s.logfConn(conn, logger.Exploit, "unknown node operation %d", operation[0])
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
	s.serveCtrl(tag, newConn(s.ctx.logger, conn, connUsageServeCtrl))
}

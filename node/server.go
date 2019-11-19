package node

import (
	"encoding/base64"
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

	guid         *guid.GUID
	listeners    map[string]*Listener // key = tag
	listenersRWM sync.RWMutex
	conns        map[string]*xnet.Conn // key = guid
	connsRWM     sync.RWMutex

	ctrlConns      map[string]*conn
	ctrlConnsRWM   sync.RWMutex
	nodeConns      map[string]*conn
	nodeConnsRWM   sync.RWMutex
	beaconConns    map[string]*conn
	beaconConnsRWM sync.RWMutex

	rand *random.Rand

	inShutdown int32
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

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

	// decrypt configs about listeners
	if len(cfg.AESCrypto) != aes.Key256Bit+aes.IVSize {
		return nil, errors.New("invalid aes key")
	}
	aesKey := cfg.AESCrypto[:aes.Key256Bit]
	aesIV := cfg.AESCrypto[aes.Key256Bit:]
	defer func() {
		security.FlushBytes(aesKey)
		security.FlushBytes(aesIV)
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

	server := server{listeners: make(map[string]*Listener)}
	for i := 0; i < len(listeners); i++ {
		_, err = server.addListener(listeners[i])
		if err != nil {
			return nil, err
		}
	}

	server.ctx = ctx
	server.maxConns = cfg.MaxConns
	server.timeout = cfg.Timeout
	server.guid = guid.New(1024, server.ctx.global.Now)
	server.conns = make(map[string]*xnet.Conn)
	server.ctrlConns = make(map[string]*conn)
	server.nodeConns = make(map[string]*conn)
	server.beaconConns = make(map[string]*conn)
	server.rand = random.New(0)
	server.stopSignal = make(chan struct{})
	return &server, nil
}

// Deploy is used to deploy added listener
func (server *server) Deploy() error {
	// deploy all listener
	l := len(server.listeners)
	errs := make(chan error, l)
	for tag, l := range server.listeners {
		go func(tag string, l *Listener) {
			errs <- server.deploy(tag, l)
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

// Close is used to close all listeners and connections
func (server *server) Close() {
	atomic.StoreInt32(&server.inShutdown, 1)
	close(server.stopSignal)
	// close all listeners
	server.listenersRWM.RLock()
	defer server.listenersRWM.RUnlock()
	for _, listener := range server.listeners {
		_ = listener.Close()
	}
	// close all conns
	server.connsRWM.Lock()
	defer server.connsRWM.Unlock()
	for _, conn := range server.conns {
		_ = conn.Close()
	}
	server.wg.Wait()
	server.ctx = nil
}

func (server *server) addListener(l *messages.Listener) (*Listener, error) {
	tlsConfig, err := l.TLSConfig.Apply()
	if err != nil {
		return nil, errors.Wrapf(err, "listener %s", l.Tag)
	}
	tlsConfig.Time = server.ctx.global.Now // <security>
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
	listener = netutil.LimitListener(listener, server.maxConns)
	ll := &Listener{Mode: l.Mode, Listener: listener}
	server.listenersRWM.Lock()
	defer server.listenersRWM.Unlock()
	if _, ok := server.listeners[l.Tag]; !ok {
		server.listeners[l.Tag] = ll
		return ll, nil
	}
	return nil, errors.Errorf("listener %s already exists", l.Tag)
}

func (server *server) deploy(tag string, listener *Listener) error {
	errChan := make(chan error, 1)
	server.wg.Add(1)
	go server.serve(tag, listener, errChan)
	select {
	case err := <-errChan:
		const format = "failed to deploy listener %s(%s): %s"
		return errors.Errorf(format, tag, listener.Addr(), err)
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (server *server) serve(tag string, l *Listener, errChan chan<- error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "server.serve()")
			server.log(logger.Fatal, err)
		}
		errChan <- err
		close(errChan)
		// delete
		server.listenersRWM.Lock()
		delete(server.listeners, tag)
		server.listenersRWM.Unlock()
		server.logf(logger.Info, "listener: %s(%s) is closed", tag, l.Addr())
		server.wg.Done()
	}()
	var delay time.Duration // how long to sleep on accept failure
	maxDelay := 2 * time.Second
	for {
		conn, e := l.Accept()
		if e != nil {
			select {
			case <-server.stopSignal:
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
				server.logf(logger.Warning, "accept error: %s; retrying in %v", e, delay)
				time.Sleep(delay)
				continue
			}
			if !strings.Contains(e.Error(), "use of closed network connection") {
				err = e
			}
			return
		}
		delay = 0
		server.wg.Add(1)
		go server.handshake(tag, conn)
	}
}

func (server *server) shuttingDown() bool {
	return atomic.LoadInt32(&server.inShutdown) != 0
}

func (server *server) AddListener(l *messages.Listener) error {
	if server.shuttingDown() {
		return errors.WithStack(errServerClosed)
	}
	listener, err := server.addListener(l)
	if err != nil {
		return err
	}
	return server.deploy(l.Tag, listener)
}

func (server *server) Listeners() map[string]*Listener {
	return nil
}

func (server *server) GetListener(tag string) *Listener {
	return nil
}

func (server *server) CloseListener(tag string) {

}

func (server *server) CloseConn(address string) {

}

func (server *server) logf(lv logger.Level, format string, log ...interface{}) {
	server.ctx.logger.Printf(lv, "server", format, log...)
}

func (server *server) log(lv logger.Level, log ...interface{}) {
	server.ctx.logger.Print(lv, "server", log...)
}

// checkConn is used to check connection is from client
func (server *server) checkConn(conn net.Conn) bool {
	size := byte(100 + server.rand.Int(156))
	data := server.rand.Bytes(int(size))
	_, err := conn.Write(append([]byte{size}, data...))
	if err != nil {
		server.logf(logger.Warning, "failed to send check data: %s", err)
		return false
	}
	n, err := io.ReadFull(conn, data)
	if err != nil {
		server.logf(logger.Warning, "test data: %X", data[:n])
		return false
	}
	return true
}

func (server *server) sendCertificate(conn *xnet.Conn) bool {
	var err error
	cert := server.ctx.global.Certificate()
	if cert != nil {
		err = conn.Send(cert)
		if err != nil {
			server.logf(logger.Error, "failed to send certificate: %s", err)
			return false
		}
	} else { // if no certificate send padding data
		size := 1024 + server.rand.Int(1024)
		err = conn.Send(server.rand.Bytes(size))
		if err != nil {
			server.logf(logger.Error, "failed to send padding data: %s", err)
			return false
		}
	}
	return true
}

func (server *server) handshake(tag string, conn net.Conn) {
	now := server.ctx.global.Now()
	xConn := xnet.NewConn(conn, now)
	defer func() {
		if r := recover(); r != nil {
			server.log(logger.Exploit, xpanic.Error(r, "server.handshake"))
		}
		_ = xConn.Close()
		server.wg.Done()
	}()

	// add to server.conns for management
	connTag := tag + hex.EncodeToString(server.guid.Get())
	server.addConn(connTag, xConn)
	defer server.deleteConn(connTag)

	_ = xConn.SetDeadline(now.Add(server.timeout))

	if !server.checkConn(xConn) {
		return
	}
	if !server.sendCertificate(xConn) {
		return
	}

	// receive role
	role := make([]byte, 1)
	_, err := io.ReadFull(xConn, role)
	if err != nil {
		server.log(logger.Error, "failed to receive role")
		return
	}
	r := protocol.Role(role[0])
	switch r {
	case protocol.Beacon:
		server.verifyBeacon(xConn)
	case protocol.Node:
		server.verifyNode(xConn)
	case protocol.Ctrl:
		server.verifyCtrl(xConn)
	default:
		server.log(logger.Exploit, r)
	}
}

func (server *server) verifyBeacon(conn net.Conn) {

}

func (server *server) verifyNode(conn net.Conn) {

}

func (server *server) verifyCtrl(conn *xnet.Conn) {
	// <danger>
	// send random challenge code(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and controller sign it
	challenge := server.rand.Bytes(2048 + server.rand.Int(2048))
	err := conn.Send(challenge)
	if err != nil {
		l := &sLog{c: conn, l: "send challenge code failed", e: err}
		server.log(logger.Error, l)
		return
	}
	// receive signature
	signature, err := conn.Receive()
	if err != nil {
		l := &sLog{c: conn, l: "receive signature failed", e: err}
		server.log(logger.Error, l)
		return
	}
	// verify signature
	if !server.ctx.global.CtrlVerify(challenge, signature) {
		l := &sLog{c: conn, l: "invalid controller signature", e: err}
		server.log(logger.Exploit, l)
		return
	}
	// send success
	err = conn.Send(protocol.AuthSucceed)
	if err != nil {
		l := &sLog{c: conn, l: "send auth success response failed", e: err}
		server.log(logger.Error, l)
		return
	}
	server.serveCtrl(conn)
}

func (server *server) addConn(tag string, conn *xnet.Conn) {
	server.connsRWM.Lock()
	server.conns[tag] = conn
	server.connsRWM.Unlock()
}

func (server *server) deleteConn(tag string) {
	server.connsRWM.Lock()
	delete(server.conns, tag)
	server.connsRWM.Unlock()
}

func (server *server) addCtrlConn(tag string, conn *conn) {
	server.ctrlConnsRWM.Lock()
	if _, ok := server.ctrlConns[tag]; !ok {
		server.ctrlConns[tag] = conn
	}
	server.ctrlConnsRWM.Unlock()
}

func (server *server) deleteCtrlConn(tag string) {
	server.ctrlConnsRWM.Lock()
	delete(server.ctrlConns, tag)
	server.ctrlConnsRWM.Unlock()
}

func (server *server) addNodeConn(guid []byte, conn *conn) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.nodeConnsRWM.Lock()
	if _, ok := server.nodeConns[tag]; !ok {
		server.nodeConns[tag] = conn
	}
	server.nodeConnsRWM.Unlock()
}

func (server *server) deleteNodeConn(guid []byte) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.nodeConnsRWM.Lock()
	delete(server.nodeConns, tag)
	server.nodeConnsRWM.Unlock()
}

func (server *server) addBeaconConn(guid []byte, conn *conn) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.beaconConnsRWM.Lock()
	if _, ok := server.beaconConns[tag]; !ok {
		server.beaconConns[tag] = conn
	}
	server.beaconConnsRWM.Unlock()
}

func (server *server) deleteBeaconConn(guid []byte) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.beaconConnsRWM.Lock()
	delete(server.beaconConns, tag)
	server.beaconConnsRWM.Unlock()
}

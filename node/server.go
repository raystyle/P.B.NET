package node

import (
	"bytes"
	"encoding/base64"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/config"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
	"project/internal/xpanic"
)

var (
	errServerClosed = errors.New("server closed")
)

// accept beacon node controller
type server struct {
	ctx          *NODE
	connLimit    int           // every listener
	hsTimeout    time.Duration // handshake timeout
	listeners    map[string]*listener
	listenersRWM sync.RWMutex
	conns        map[string]*xnet.Conn // key = listener.Tag + Remote Address
	connsRWM     sync.RWMutex
	ctrls        map[string]*ctrlConn // key = base64(sha256(Remote Address))
	ctrlsRWM     sync.RWMutex
	nodes        map[string]*nodeConn // key = base64(guid)
	nodesRWM     sync.RWMutex
	beacons      map[string]*beaconConn // key = base64(guid)
	beaconsRWM   sync.RWMutex
	inShutdown   int32
	random       *random.Rand
	stopSignal   chan struct{}
	wg           sync.WaitGroup
}

type listener struct {
	Mode     xnet.Mode
	sTimeout time.Duration // start timeout
	net.Listener
}

func newServer(ctx *NODE, cfg *Config) (*server, error) {
	s := &server{
		ctx:       ctx,
		connLimit: cfg.ConnLimit,
		hsTimeout: cfg.HandshakeTimeout,
		listeners: make(map[string]*listener),
	}
	if s.connLimit < 1 {
		s.connLimit = options.DefaultConnectionLimit
	}
	if s.hsTimeout < 1 {
		s.hsTimeout = options.DefaultHandshakeTimeout
	}
	for _, listener := range cfg.Listeners {
		_, err := s.addListener(listener)
		if err != nil {
			return nil, err
		}
	}
	s.conns = make(map[string]*xnet.Conn)
	s.ctrls = make(map[string]*ctrlConn)
	s.nodes = make(map[string]*nodeConn)
	s.beacons = make(map[string]*beaconConn)
	s.random = random.New(0)
	s.stopSignal = make(chan struct{})
	return s, nil
}

func (server *server) Deploy() error {
	// deploy all listener
	l := len(server.listeners)
	errs := make(chan error, l)
	for tag, l := range server.listeners {
		go func(tag string, l *listener) {
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

func (server *server) AddListener(l *config.Listener) error {
	listener, err := server.addListener(l)
	if err != nil {
		return err
	}
	return server.deploy(l.Tag, listener)
}

func (server *server) addListener(l *config.Listener) (*listener, error) {
	if server.shuttingDown() {
		return nil, errServerClosed
	}
	c := &xnet.Config{}
	err := toml.Unmarshal(l.Config, c)
	if err != nil {
		return nil, errors.Errorf("load listener %s config failed: %s", l.Tag, err)
	}
	li, err := xnet.Listen(l.Mode, c)
	if err != nil {
		return nil, errors.Errorf("listen %s failed: %s", l.Tag, err)
	}
	li = netutil.LimitListener(li, server.connLimit)
	listener := &listener{Mode: l.Mode, sTimeout: l.Timeout, Listener: li}
	// add
	server.listenersRWM.Lock()
	if _, exist := server.listeners[l.Tag]; !exist {
		server.listeners[l.Tag] = listener
		server.listenersRWM.Unlock()
	} else {
		server.listenersRWM.Unlock()
		return nil, errors.Errorf("listener: %s already exists", l.Tag)
	}
	return listener, nil
}

func (server *server) deploy(tag string, l *listener) error {
	timeout := l.sTimeout
	if timeout < 1 {
		timeout = options.DefaultStartTimeout
	}
	addr := l.Addr().String()
	errChan := make(chan error, 1)
	server.wg.Add(1)
	go server.serve(tag, l, errChan)
	select {
	case err := <-errChan:
		return errors.Errorf("listener: %s(%s) deploy failed: %s", tag, addr, err)
	case <-time.After(timeout):
		return nil
	}
}

func (server *server) serve(tag string, l *listener, errChan chan<- error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error("serve panic:", r) // front var err
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
	max := 2 * time.Second
	for {
		conn, e := l.Accept()
		if e != nil {
			select {
			case <-server.stopSignal:
				err = errServerClosed
				return
			default:
			}
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if delay > max {
					delay = max
				}
				server.logf(logger.Warning, "accept error: %s; retrying in %v", e, delay)
				time.Sleep(delay)
				continue
			}
			return
		}
		delay = 0
		server.wg.Add(1)
		go server.handshake(tag, conn)
	}
}

func (server *server) GetListener(tag string) net.Listener {
	return nil
}

func (server *server) Listeners(tag string) map[string]net.Listener {
	return nil
}

func (server *server) CloseListener(tag string) {

}

func (server *server) CloseConn(address string) {

}

func (server *server) Close() {
	atomic.StoreInt32(&server.inShutdown, 1)
	close(server.stopSignal)
	// close all listeners
	server.listenersRWM.Lock()
	for _, listener := range server.listeners {
		_ = listener.Close()
	}
	server.listenersRWM.Unlock()
	// close all conns
	server.connsRWM.Lock()
	for _, conn := range server.conns {
		_ = conn.Close()
	}
	server.connsRWM.Unlock()
	server.wg.Wait()
}

func (server *server) logf(l logger.Level, format string, log ...interface{}) {
	server.ctx.Printf(l, "server", format, log...)
}

func (server *server) log(l logger.Level, log ...interface{}) {
	server.ctx.Print(l, "server", log...)
}

func (server *server) logln(l logger.Level, log ...interface{}) {
	server.ctx.Println(l, "server", log...)
}

func (server *server) shuttingDown() bool {
	return atomic.LoadInt32(&server.inShutdown) != 0
}

// serve log(handshake client)
type sLog struct {
	c net.Conn
	l string
	e error
}

func (sl *sLog) String() string {
	b := logger.Conn(sl.c)
	b.WriteString(sl.l)
	if sl.e != nil {
		b.WriteString(": ")
		b.WriteString(sl.e.Error())
	}
	return b.String()
}

func (server *server) handshake(lTag string, conn net.Conn) {
	dConn := xnet.NewDeadlineConn(conn, server.hsTimeout)
	xconn := xnet.NewConn(dConn, server.ctx.global.Now())
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("handshake panic:", r)
			server.log(logger.Exploit, &sLog{c: xconn, e: err})
		}
		_ = xconn.Close()
		server.wg.Done()
	}()
	// conn tag
	b := bytes.Buffer{}
	b.WriteString(lTag)
	b.WriteString(xconn.RemoteAddr().String())
	connTag := b.String()
	// add to conns for management
	server.addConn(connTag, xconn)
	defer server.delConn(connTag)
	// send certificate
	var err error
	cert := server.ctx.global.Certificate()
	if cert != nil {
		err = xconn.Send(cert)
		if err != nil {
			l := &sLog{c: xconn, l: "send certificate failed", e: err}
			server.log(logger.Error, l)
			return
		}
	} else { // if no certificate send padding data
		paddingSize := 1024 + server.random.Int(1024)
		err = xconn.Send(server.random.Bytes(paddingSize))
		if err != nil {
			l := &sLog{c: xconn, l: "send padding data failed", e: err}
			server.log(logger.Error, l)
			return
		}
	}
	// receive role
	role := make([]byte, 1)
	_, err = io.ReadFull(xconn, role)
	if err != nil {
		l := &sLog{c: xconn, l: "receive role failed", e: err}
		server.log(logger.Error, l)
		return
	}
	// remove deadline conn
	xconn = xnet.NewConn(conn, server.ctx.global.Now())
	r := protocol.Role(role[0])
	switch r {
	case protocol.Beacon:
		server.verifyBeacon(xconn)
	case protocol.Node:
		server.verifyNode(xconn)
	case protocol.Ctrl:
		server.verifyCtrl(xconn)
	default:
		server.log(logger.Exploit, &sLog{c: xconn, e: r})
	}
}

func (server *server) verifyBeacon(conn *xnet.Conn) {

}

func (server *server) verifyNode(conn *xnet.Conn) {

}

func (server *server) verifyCtrl(conn *xnet.Conn) {
	dConn := xnet.NewDeadlineConn(conn, server.hsTimeout)
	xconn := xnet.NewConn(dConn, server.ctx.global.Now())
	// <danger>
	// send random challenge code(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and controller sign it
	challenge := server.random.Bytes(2048 + server.random.Int(2048))
	err := xconn.Send(challenge)
	if err != nil {
		l := &sLog{c: xconn, l: "send challenge code failed", e: err}
		server.log(logger.Error, l)
		return
	}
	// receive signature
	signature, err := xconn.Receive()
	if err != nil {
		l := &sLog{c: xconn, l: "receive signature failed", e: err}
		server.log(logger.Error, l)
		return
	}
	// verify signature
	if !server.ctx.global.CtrlVerify(challenge, signature) {
		l := &sLog{c: xconn, l: "invalid controller signature", e: err}
		server.log(logger.Exploit, l)
		return
	}
	// send success
	err = xconn.Send(protocol.AuthSucceed)
	if err != nil {
		l := &sLog{c: xconn, l: "send auth success response failed", e: err}
		server.log(logger.Error, l)
		return
	}
	server.serveCtrl(conn)
}

func (server *server) addConn(tag string, c *xnet.Conn) {
	server.connsRWM.Lock()
	server.conns[tag] = c
	server.connsRWM.Unlock()
}

func (server *server) delConn(tag string) {
	server.connsRWM.Lock()
	delete(server.conns, tag)
	server.connsRWM.Unlock()
}

func (server *server) addCtrl(tag string, ctrl *ctrlConn) {
	server.ctrlsRWM.Lock()
	if _, ok := server.ctrls[tag]; !ok {
		server.ctrls[tag] = ctrl
	}
	server.ctrlsRWM.Unlock()
}

func (server *server) delCtrl(tag string) {
	server.ctrlsRWM.Lock()
	delete(server.ctrls, tag)
	server.ctrlsRWM.Unlock()
}

func (server *server) addNode(guid []byte, node *nodeConn) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.nodesRWM.Lock()
	if _, ok := server.nodes[tag]; !ok {
		server.nodes[tag] = node
	}
	server.nodesRWM.Unlock()
}

func (server *server) delNode(guid []byte) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.nodesRWM.Lock()
	delete(server.nodes, tag)
	server.nodesRWM.Unlock()
}

func (server *server) addBeacon(guid []byte, beacon *beaconConn) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.beaconsRWM.Lock()
	if _, ok := server.beacons[tag]; !ok {
		server.beacons[tag] = beacon
	}
	server.beaconsRWM.Unlock()
}

func (server *server) delBeacon(guid []byte) {
	tag := base64.StdEncoding.EncodeToString(guid)
	server.beaconsRWM.Lock()
	delete(server.beacons, tag)
	server.beaconsRWM.Unlock()
}

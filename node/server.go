package node

import (
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/config"
	"project/internal/crypto/sha256"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/random"
	"project/internal/xnet"
)

var (
	ERR_SERVER_CLOSED = errors.New("server closed")
)

// accept beacon node controller
type server struct {
	ctx           *NODE
	conn_limit    int           // every listener
	hs_timeout    time.Duration // handshake timeout
	listeners     map[string]*listener
	listeners_rwm sync.RWMutex
	conns         map[string]*xnet.Conn // key = listener.Tag + Remote Address
	conns_rwm     sync.RWMutex
	ctrls         map[string]v_client // key = base64(sha256(Remote Address))
	ctrls_rwm     sync.RWMutex
	nodes         map[string]v_client // key = base64(guid)
	nodes_rwm     sync.RWMutex
	beacons       map[string]v_client // key = base64(guid)
	beacons_rwm   sync.RWMutex
	in_shutdown   int32
	random        *random.Generator
	stop_signal   chan struct{}
	wg            sync.WaitGroup
}

type listener struct {
	Mode      xnet.Mode
	s_timeout time.Duration // start timeout
	net.Listener
}

type v_client interface {
	Info() *xnet.Info
	Close()
	Kill()
}

func new_server(ctx *NODE, c *Config) (*server, error) {
	s := &server{
		ctx:        ctx,
		conn_limit: c.Conn_Limit,
		hs_timeout: c.Handshake_Timeout,
		listeners:  make(map[string]*listener),
	}
	if s.conn_limit < 1 {
		s.conn_limit = options.DEFAULT_CONNECTION_LIMIT
	}
	if s.hs_timeout < 1 {
		s.hs_timeout = options.DEFAULT_HANDSHAKE_TIMEOUT
	}
	for _, listener := range c.Listeners {
		_, err := s.add_listener(listener)
		if err != nil {
			return nil, err
		}
	}
	s.conns = make(map[string]*xnet.Conn)
	s.ctrls = make(map[string]v_client)
	s.nodes = make(map[string]v_client)
	s.beacons = make(map[string]v_client)
	s.random = random.New()
	s.stop_signal = make(chan struct{}, 1)
	return s, nil
}

func (this *server) Deploy() error {
	// deploy all listener
	errs := make(chan error, len(this.listeners))
	for tag, l := range this.listeners {
		go func(tag string, l *listener) {
			errs <- this.deploy(tag, l)
		}(tag, l)
	}
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *server) Add_Listener(l *config.Listener) error {
	listener, err := this.add_listener(l)
	if err != nil {
		return err
	}
	return this.deploy(l.Tag, listener)
}

func (this *server) add_listener(l *config.Listener) (*listener, error) {
	if this.shutting_down() {
		return nil, ERR_SERVER_CLOSED
	}
	c := &xnet.Config{}
	err := toml.Unmarshal(l.Config, c)
	if err != nil {
		return nil, errors.Errorf("load %s config failed: %s", l.Tag, err)
	}
	li, err := xnet.Listen(l.Mode, c)
	if err != nil {
		return nil, errors.Errorf("listen %s failed: %s", l.Tag, err)
	}
	li = netutil.LimitListener(li, this.conn_limit)
	listener := &listener{Mode: l.Mode, s_timeout: l.Timeout, Listener: li}
	// add
	this.listeners_rwm.Lock()
	if _, exist := this.listeners[l.Tag]; !exist {
		this.listeners[l.Tag] = listener
		this.listeners_rwm.Unlock()
	} else {
		this.listeners_rwm.Unlock()
		err = fmt.Errorf("listener: %s already exists", l.Tag)
		return nil, err
	}
	return listener, nil
}

func (this *server) deploy(tag string, l *listener) error {
	timeout := l.s_timeout
	if timeout < 1 {
		timeout = options.DEFAULT_START_TIMEOUT
	}
	addr := l.Addr().String()
	err_chan := make(chan error, 1)
	this.wg.Add(1)
	go this.serve(tag, l, err_chan)
	select {
	case err := <-err_chan:
		return fmt.Errorf("listener: %s(%s) deploy failed: %s", tag, addr, err)
	case <-time.After(timeout):
		return nil
	}
}

func (this *server) serve(tag string, l *listener, err_chan chan<- error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("serve panic: %v", r))
		}
		err_chan <- err
		close(err_chan)
		// delete
		this.listeners_rwm.Lock()
		delete(this.listeners, tag)
		this.listeners_rwm.Unlock()
		this.logf(logger.INFO, "listener: %s(%s) is closed", tag, l.Addr())
		this.wg.Done()
	}()
	var delay time.Duration // how long to sleep on accept failure
	max := 2 * time.Second
	for {
		conn, e := l.Accept()
		if e != nil {
			select {
			case <-this.stop_signal:
				err = ERR_SERVER_CLOSED
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
				this.logf(logger.WARNING, "accept error: %s; retrying in %v", e, delay)
				time.Sleep(delay)
				continue
			}
			return
		}
		delay = 0
		this.wg.Add(1)
		_ = conn.SetDeadline(time.Now().Add(this.hs_timeout))
		go this.handshake(tag, conn)
	}
}

func (this *server) Get_Listener(tag string) net.Listener {
	return nil
}

func (this *server) Listeners(tag string) map[string]net.Listener {
	return nil
}

func (this *server) Close_Listener(tag string) {

}

func (this *server) Close_Conn(address string) {

}

func (this *server) Shutdown() {
	atomic.StoreInt32(&this.in_shutdown, 1)
	close(this.stop_signal)
	// close all listeners
	this.listeners_rwm.Lock()
	for _, listener := range this.listeners {
		_ = listener.Close()
	}
	this.listeners_rwm.Unlock()
	// close all conns
	this.conns_rwm.Lock()
	for _, conn := range this.conns {
		_ = conn.Close()
	}
	this.conns_rwm.Unlock()
	this.wg.Wait()
}

func (this *server) logf(l logger.Level, format string, log ...interface{}) {
	this.ctx.Printf(l, "server", format, log...)
}

func (this *server) log(l logger.Level, log ...interface{}) {
	this.ctx.Print(l, "server", log...)
}

func (this *server) logln(l logger.Level, log ...interface{}) {
	this.ctx.Println(l, "server", log...)
}

func (this *server) shutting_down() bool {
	return atomic.LoadInt32(&this.in_shutdown) != 0
}

func (this *server) add_conn(tag string, c *xnet.Conn) {
	this.conns_rwm.Lock()
	this.conns[tag] = c
	this.conns_rwm.Unlock()
}

func (this *server) del_conn(tag string) {
	this.conns_rwm.Lock()
	delete(this.conns, tag)
	this.conns_rwm.Unlock()
}

func (this *server) add_ctrl(c v_client) {
	data := sha256.Bytes([]byte(c.Info().Remote_Address))
	tag := base64.StdEncoding.EncodeToString(data)
	this.ctrls_rwm.Lock()
	if _, exist := this.ctrls[tag]; !exist {
		this.ctrls[tag] = c
	}
	this.ctrls_rwm.Unlock()
}

func (this *server) del_ctrl(tag string, c v_client) {
	if c != nil {
		data := sha256.Bytes([]byte(c.Info().Remote_Address))
		tag = base64.StdEncoding.EncodeToString(data)
	}
	this.ctrls_rwm.Lock()
	delete(this.ctrls, tag)
	this.ctrls_rwm.Unlock()
}

func (this *server) add_node(guid []byte, c v_client) {
	tag := base64.StdEncoding.EncodeToString(guid)
	this.nodes_rwm.Lock()
	if _, exist := this.nodes[tag]; !exist {
		this.nodes[tag] = c
	}
	this.nodes_rwm.Unlock()
}

func (this *server) del_node(guid []byte) {
	tag := base64.StdEncoding.EncodeToString(guid)
	this.nodes_rwm.Lock()
	delete(this.nodes, tag)
	this.nodes_rwm.Unlock()
}

func (this *server) add_beacon(guid []byte, c v_client) {
	tag := base64.StdEncoding.EncodeToString(guid)
	this.beacons_rwm.Lock()
	if _, exist := this.beacons[tag]; !exist {
		this.beacons[tag] = c
	}
	this.beacons_rwm.Unlock()
}

func (this *server) del_beacon(guid []byte) {
	tag := base64.StdEncoding.EncodeToString(guid)
	this.beacons_rwm.Lock()
	delete(this.beacons, tag)
	this.beacons_rwm.Unlock()
}

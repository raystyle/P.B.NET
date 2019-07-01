package node

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/config"
	"project/internal/convert"
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
	conn_limit    int // every listener
	listeners     map[string]*listener
	listeners_rwm sync.RWMutex
	conns         map[string]*conn // key = listener.Tag + Remote Address
	conns_rwm     sync.RWMutex
	in_shutdown   int32
	random        *random.Generator
	stop_signal   chan struct{}
	wg            sync.WaitGroup
}

type listener struct {
	Mode xnet.Mode
	net.Listener
}

func new_server(ctx *NODE, c *Config) (*server, error) {
	s := &server{
		ctx:         ctx,
		conn_limit:  c.Conn_Limit,
		listeners:   make(map[string]*listener),
		conns:       make(map[string]*conn),
		random:      random.New(),
		stop_signal: make(chan struct{}, 1),
	}
	if s.conn_limit < 1 {
		s.conn_limit = options.DEFAULT_CONNECTION_LIMIT
	}
	for _, listener := range c.Listeners {
		err := s.Serve(listener)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (this *server) Deploy() error {
	return nil
}

func (this *server) Serve(l *config.Listener) error {
	c := &xnet.Config{}
	err := toml.Unmarshal(l.Config, c)
	if err != nil {
		this.logf(logger.INFO, "load %s config failed: %s", l.Tag, err)
		return err
	}
	li, err := xnet.Listen(l.Mode, c)
	if err != nil {
		this.logf(logger.INFO, "listen %s failed: %s", l.Tag, err)
		return err
	}
	li = netutil.LimitListener(li, this.conn_limit)
	addr := li.Addr().String()
	listener := &listener{Mode: l.Mode, Listener: li}
	err = this.track_listener(l.Tag, listener, true)
	if err != nil {
		this.logf(logger.INFO, "track listener %s failed: %s", l.Tag, err)
		return err
	}
	timeout := c.Timeout
	if timeout < 1 {
		timeout = options.DEFAULT_START_TIMEOUT
	}
	err_chan := make(chan error, 1)
	this.wg.Add(1)
	go this.serve(l.Tag, listener, err_chan)
	select {
	case err := <-err_chan:
		this.logf(logger.INFO, "listener: %s(%s) serve failed: %s", l.Tag, addr, err)
		return err
	case <-time.After(timeout):
		this.logf(logger.INFO, "listener: %s(%s) is serving", l.Tag, addr)
		return nil
	}
}

func (this *server) serve(tag string, l *listener, err_chan chan<- error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = errors.WithStack(v)
			default:
				err = errors.New("unknown panic")
			}
		}
		err_chan <- err
		close(err_chan)
		_ = this.track_listener(tag, l, false)
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
		go this.handshake(conn)
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
	for _, l := range this.listeners {
		_ = l.Close()
	}
	this.listeners_rwm.Unlock()
	// close all conns
	this.conns_rwm.Lock()
	for _, c := range this.conns {
		_ = c.Close()
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

func (this *server) track_listener(tag string, l *listener, add bool) error {
	this.listeners_rwm.Lock()
	defer this.listeners_rwm.Unlock()
	if add {
		if this.shutting_down() {
			return ERR_SERVER_CLOSED
		}
		if _, exist := this.listeners[tag]; !exist {
			this.listeners[tag] = l
		} else {
			return fmt.Errorf("listener: %s already exists", tag)
		}
	} else {
		if _, exist := this.listeners[tag]; exist {
			delete(this.listeners, tag)
		} else {
			return fmt.Errorf("listener: %s doesn't exist", tag)
		}
	}
	return nil
}

func (this *server) track_conn(tag string, c *conn, add bool) error {
	this.conns_rwm.Lock()
	defer this.conns_rwm.Unlock()
	if add {
		if this.shutting_down() {
			return ERR_SERVER_CLOSED
		}
		this.conns[tag] = c
		// if _, exist := this.conns[tag]; !exist {
		//
		// } else {
		// 	return fmt.Errorf("conn: %s already exists", tag)
		// }
	} else {
		delete(this.conns, tag)
		// if _, exist := this.conns[tag]; exist {
		//
		// } else {
		// 	return fmt.Errorf("conn: %s doesn't exist", tag)
		// }
	}
	return nil
}

type conn struct {
	net.Conn
	connect   int64 // timestamp
	l_network string
	l_address string
	r_network string
	r_address string
	version   uint32
	send      int // imprecise
	receive   int // imprecise
	rwm       sync.RWMutex
}

func (this *conn) Read(b []byte) (int, error) {
	n, err := this.Conn.Read(b)
	this.rwm.Lock()
	this.receive += n
	this.rwm.Unlock()
	if err != nil {
		return n, err
	}
	return n, nil
}

func (this *conn) Write(b []byte) (int, error) {
	n, err := this.Conn.Write(b)
	this.rwm.Lock()
	this.send += n
	this.rwm.Unlock()
	if err != nil {
		return n, err
	}
	return n, nil
}

func (this *conn) Info() *xnet.Info {
	this.rwm.RLock()
	i := &xnet.Info{
		Send:    this.send,
		Receive: this.receive,
	}
	this.rwm.RUnlock()
	i.Connect_Time = this.connect
	i.Local_Network = this.l_network
	i.Local_Address = this.l_address
	i.Remote_Network = this.r_network
	i.Remote_Address = this.r_address
	return i
}

func (this *conn) send_msg(msg []byte) error {
	size := convert.Uint32_Bytes(uint32(len(msg)))
	_, err := this.Write(append(size, msg...))
	return err
}

func (this *conn) recv_msg() ([]byte, error) {
	size := make([]byte, 4)
	_, err := io.ReadFull(this, size)
	if err != nil {
		return nil, err
	}
	s := convert.Bytes_Uint32(size)
	msg := make([]byte, int(s))
	_, err = io.ReadFull(this, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

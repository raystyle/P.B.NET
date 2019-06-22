package node

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/messages"
	"project/internal/options"
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
	conns         map[string]*conn
	conns_rwm     sync.RWMutex
	in_shutdown   int32
	stop_signal   chan struct{}
}

type listener struct {
	Mode xnet.Mode
	net.Listener
}

func new_server(ctx *NODE) (*server, error) {
	s := &server{
		ctx:         ctx,
		conn_limit:  ctx.config.Conn_Limit,
		listeners:   make(map[string]*listener),
		conns:       make(map[string]*conn),
		stop_signal: make(chan struct{}, 1),
	}
	if s.conn_limit < 1 {
		s.conn_limit = options.DEFAULT_CONNECTION_LIMIT
	}
	for _, listener := range ctx.config.Listeners {
		err := s.Serve(listener)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (this *server) Serve(l *messages.Listener) error {
	li, err := xnet.Listen(l.Mode, l.Config)
	if err != nil {
		return err
	}
	li = netutil.LimitListener(li, this.conn_limit)
	listener := &listener{Mode: l.Mode, Listener: li}
	err = this.track_listener(l.Tag, listener, true)
	if err != nil {
		return err
	}
	timeout := l.Timeout
	if timeout < 1 {
		timeout = options.DEFAULT_START_TIMEOUT
	}
	addr := li.Addr().String()
	err_chan := make(chan error, 1)
	go this.serve(l.Tag, listener, err_chan)
	select {
	case err := <-err_chan:
		this.logf(logger.INFO, "%s(%s) serve failed: %s", l.Tag, addr, err)
		return err
	case <-time.After(timeout):
		this.logf(logger.INFO, "%s(%s) is serving", l.Tag, addr)
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
		this.new_conn(conn)
	}
}

func (this *server) Get_Listener(tag string) net.Listener {
	return nil
}

func (this *server) Listeners(tag string) map[string]net.Listener {
	return nil
}

func (this *server) Kill_Listener(tag string) {

}

func (this *server) Kill_Conn(address string) {

}

func (this *server) Shutdown() {
	atomic.StoreInt32(&this.in_shutdown, 1)
	close(this.stop_signal)
}

func (this *server) log(l logger.Level, log ...interface{}) {
	this.ctx.logger.Println(l, "server", log...)
}

func (this *server) logf(l logger.Level, format string, log ...interface{}) {
	this.ctx.logger.Printf(l, "server", format, log...)
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

type conn struct {
	conn net.Conn
}

func (this *server) new_conn(c net.Conn) {
	time.Sleep(10 * time.Second)
	_ = c.Close()
}

package node

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xnet"
)

// accept beacon node controller
type server struct {
	ctx           *NODE
	start_timeout time.Duration
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
		listeners:   make(map[string]*listener),
		conns:       make(map[string]*conn),
		stop_signal: make(chan struct{}, 1),
	}
	for tag, config := range ctx.config.Listeners {
		l, err := xnet.Listen(config)
		if err != nil {
			return nil, errors.Wrapf(err, "listen %s failed", tag)
		}
		go s.Serve(tag, &listener{Mode: config.Mode, Listener: l})
	}
	return s, nil
}

func (this *server) Serve(tag string, l *listener) error {
	if this.shutting_down() {
		return errors.New("server closed")
	}
	defer this.listeners_rwm.Unlock()
	this.listeners_rwm.Lock()
	if _, exist := this.listeners[tag]; !exist {
		this.listeners[tag] = l
	} else {
		return fmt.Errorf("listener: %s already exists", tag)
	}
	var delay time.Duration // how long to sleep on accept failure
	max := 2 * time.Second

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-this.stop_signal:
				return errors.New("server closed")
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {

				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if delay > max {
					delay = max
				}
				this.logf(logger.WARNING, "accept error: %s; retrying in %v", err, delay)
				time.Sleep(delay)
				continue

			}

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

type conn struct {
	conn net.Conn
}

func (this *server) new_conn(c net.Conn) {

}

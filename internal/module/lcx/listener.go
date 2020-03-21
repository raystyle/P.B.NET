package lcx

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/xpanic"
)

// Listener is used to accept slave's connection and user's connection.
type Listener struct {
	iNetwork string // slave connected Listener
	iAddress string // slave connected Listener
	logger   logger.Logger
	opts     *Options

	logSrc    string
	iListener net.Listener // accept slaver connection
	lListener net.Listener // accept user connection
	conns     map[*lConn]struct{}
	rwm       sync.RWMutex

	wg sync.WaitGroup
}

// NewListener is used to create a listener.
func NewListener(
	tag string,
	iNetwork string,
	iAddress string,
	logger logger.Logger,
	opts *Options,
) (*Listener, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}
	_, err := net.ResolveTCPAddr(iNetwork, iAddress)
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = new(Options)
	}
	opts = opts.apply()
	_, err = net.ResolveTCPAddr(opts.LocalNetwork, opts.LocalAddress)
	if err != nil {
		return nil, err
	}
	return &Listener{
		iNetwork: iNetwork,
		iAddress: iAddress,
		logger:   logger,
		opts:     opts,
		logSrc:   "lcx listen-" + tag,
		conns:    make(map[*lConn]struct{}),
	}, nil
}

// Start is used to start listener.
func (l *Listener) Start() error {
	l.rwm.Lock()
	defer l.rwm.Unlock()
	if l.iListener != nil {
		return errors.New("already start lcx listen")
	}
	iListener, err := net.Listen(l.iNetwork, l.iAddress)
	if err != nil {
		return err
	}
	lListener, err := net.Listen(l.opts.LocalNetwork, l.opts.LocalAddress)
	if err != nil {
		return err
	}
	iListener = netutil.LimitListener(iListener, l.opts.MaxConns)
	lListener = netutil.LimitListener(lListener, l.opts.MaxConns)
	l.iListener = iListener
	l.lListener = lListener
	l.wg.Add(1)
	go l.serve(iListener, lListener)
	return nil
}

// Stop is used to stop listener.
func (l *Listener) Stop() {
	l.stop()
	l.wg.Wait()
}

func (l *Listener) stop() {
	l.rwm.Lock()
	defer l.rwm.Unlock()
	if l.iListener == nil {
		return
	}
	_ = l.iListener.Close()
	_ = l.lListener.Close()
	l.iListener = nil
	l.lListener = nil
	// close all connections
	for conn := range l.conns {
		_ = conn.Close()
	}
}

// Restart is used to restart listener.
func (l *Listener) Restart() error {
	l.Stop()
	return l.Start()
}

// Name is used to get the module name.
func (l *Listener) Name() string {
	return "lcx listen"
}

// Info is used to get the listener information.
// "income: tcp 0.0.0.0:1999, local: tcp 127.0.0.1:19993"
func (l *Listener) Info() string {
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	iNetwork := "unknown"
	iAddress := "unknown"
	lNetwork := "unknown"
	lAddress := "unknown"
	l.rwm.RLock()
	defer l.rwm.RUnlock()
	if l.iListener != nil {
		addr := l.iListener.Addr()
		iNetwork = addr.Network()
		iAddress = addr.String()
		addr = l.lListener.Addr()
		lNetwork = addr.Network()
		lAddress = addr.String()
	}
	const format = "income: %s %s, local: %s %s"
	_, _ = fmt.Fprintf(buf, format, iNetwork, iAddress, lNetwork, lAddress)
	return buf.String()
}

// Status is used to return the tranner status.
// connections: 12/1000 (used/limit)
func (l *Listener) Status() string {
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	l.rwm.RLock()
	defer l.rwm.RUnlock()
	const format = "connections: %d/%d (used/limit)"
	_, _ = fmt.Fprintf(buf, format, len(l.conns), l.opts.MaxConns)
	return buf.String()
}

func (l *Listener) logf(lv logger.Level, format string, log ...interface{}) {
	l.logger.Printf(lv, l.logSrc, format, log...)
}

func (l *Listener) log(lv logger.Level, log ...interface{}) {
	l.logger.Println(lv, l.logSrc, log...)
}

func (l *Listener) serve(iListener, lListener net.Listener) {
	defer func() {
		_ = iListener.Close()
		_ = lListener.Close()
	}()
	addr := iListener.Addr()
	iNetwork := addr.Network()
	iAddress := addr.String()
	addr = lListener.Addr()
	lNetwork := addr.Network()
	lAddress := addr.String()
	defer func() {
		if r := recover(); r != nil {
			l.log(logger.Fatal, xpanic.Print(r, "Listener.serve"))
		}
		const format = "income listener closed (%s %s), local listener closed (%s %s)"
		l.logf(logger.Info, format, iNetwork, iAddress, lNetwork, lAddress)
		l.wg.Done()
	}()
	const format = "start income listener (%s %s), start local listener (%s %s)"
	l.logf(logger.Info, format, iNetwork, iAddress, lNetwork, lAddress)
	// start accept
	for {
		remote := l.accept(iListener)
		if remote == nil {
			return
		}
		local := l.accept(lListener)
		if local == nil {
			_ = remote.Close()
			return
		}
		l.newConn(remote, local).Serve()
	}
}

func (l *Listener) accept(listener net.Listener) net.Conn {
	var delay time.Duration // how long to sleep on accept failure
	maxDelay := time.Second
	for {
		conn, err := listener.Accept()
		if err != nil {
			// check error
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}
				if delay > maxDelay {
					delay = maxDelay
				}
				l.logf(logger.Warning, "accept error: %s; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}
			errStr := err.Error()
			if !strings.Contains(errStr, "closed") {
				l.log(logger.Error, errStr)
			}
			return nil
		}
		return conn
	}
}

func (l *Listener) newConn(remote, local net.Conn) *lConn {
	return &lConn{
		listener: l,
		remote:   remote,
		local:    local,
	}
}

func (l *Listener) trackConn(conn *lConn, add bool) bool {
	l.rwm.Lock()
	defer l.rwm.Unlock()
	if add {
		if l.iListener == nil { // stopped
			return false
		}
		l.conns[conn] = struct{}{}
	} else {
		delete(l.conns, conn)
	}
	return true
}

type lConn struct {
	listener *Listener
	remote   net.Conn
	local    net.Conn
}

func (c *lConn) log(lv logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = logger.Conn(c.remote).WriteTo(buf)
	c.listener.log(lv, buf)
}

func (c *lConn) Serve() {
	c.listener.wg.Add(1)
	go c.serve()
}

func (c *lConn) serve() {
	const title = "lConn.serve"
	defer func() {
		if r := recover(); r != nil {
			c.log(logger.Fatal, xpanic.Print(r, title))
		}
		_ = c.remote.Close()
		_ = c.local.Close()
		c.listener.wg.Done()
	}()

	if !c.listener.trackConn(c, true) {
		return
	}
	defer c.listener.trackConn(c, false)

	c.log(logger.Info, "income connection")
	_ = c.remote.SetDeadline(time.Time{})
	_ = c.local.SetDeadline(time.Time{})
	c.listener.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				c.log(logger.Fatal, xpanic.Print(r, title))
			}
			c.listener.wg.Done()
		}()
		_, _ = io.Copy(c.local, c.remote)
	}()
	_, _ = io.Copy(c.remote, c.local)
}

func (c *lConn) Close() error {
	return c.local.Close()
}

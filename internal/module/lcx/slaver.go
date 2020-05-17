package lcx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/random"
	"project/internal/xpanic"
)

// Slaver is used to connect the target and connect the Listener.
type Slaver struct {
	lNetwork   string // Listener
	lAddress   string // Listener
	dstNetwork string // destination
	dstAddress string // destination
	logger     logger.Logger
	opts       *Options

	logSrc  string
	dialer  net.Dialer
	start   bool
	sleeper *random.Sleeper
	conns   map[*sConn]struct{}
	rwm     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSlaver is used to create a slaver.
func NewSlaver(
	tag string,
	lNetwork string,
	lAddress string,
	dstNetwork string,
	dstAddress string,
	logger logger.Logger,
	opts *Options,
) (*Slaver, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}
	_, err := net.ResolveTCPAddr(lNetwork, lAddress)
	if err != nil {
		return nil, err
	}
	_, err = net.ResolveTCPAddr(dstNetwork, dstAddress)
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = new(Options)
	}
	opts = opts.apply()
	return &Slaver{
		lNetwork:   lNetwork,
		lAddress:   lAddress,
		dstNetwork: dstNetwork,
		dstAddress: dstAddress,
		logger:     logger,
		opts:       opts,
		logSrc:     "lcx slave-" + tag,
		sleeper:    random.NewSleeper(),
		conns:      make(map[*sConn]struct{}),
	}, nil
}

// Start is used to start slaver.
func (s *Slaver) Start() error {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	if s.start {
		return errors.New("already start lcx slave")
	}
	s.start = true
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg.Add(1)
	go s.serve()
	return nil
}

// Stop is used to stop slaver.
func (s *Slaver) Stop() {
	s.stop()
	s.wg.Wait()
}

func (s *Slaver) stop() {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	if !s.start {
		return
	}
	s.cancel()
	s.start = false
	// close all connections
	for conn := range s.conns {
		err := conn.Close()
		if err != nil && !nettool.IsNetClosingError(err) {
			s.log(logger.Error, "failed to close connection:", err)
		}
		delete(s.conns, conn)
	}
}

// Restart is used to restart slaver.
func (s *Slaver) Restart() error {
	s.Stop()
	return s.Start()
}

// Name is used to get the module name.
func (s *Slaver) Name() string {
	return "lcx slave"
}

// Info is used to get the slaver information.
// "listener: tcp 0.0.0.0:1999, target: tcp 192.168.1.2:3389"
func (s *Slaver) Info() string {
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	const format = "listener: %s %s, target: %s %s"
	_, _ = fmt.Fprintf(buf, format, s.lNetwork, s.lAddress, s.dstNetwork, s.dstAddress)
	return buf.String()
}

// Status is used to return the slaver status.
// connections: 12/1000 (used/limit)
func (s *Slaver) Status() string {
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	const format = "connections: %d/%d (used/limit)"
	_, _ = fmt.Fprintf(buf, format, len(s.conns), s.opts.MaxConns)
	return buf.String()
}

func (s *Slaver) logf(lv logger.Level, format string, log ...interface{}) {
	s.logger.Printf(lv, s.logSrc, format, log...)
}

func (s *Slaver) log(lv logger.Level, log ...interface{}) {
	s.logger.Println(lv, s.logSrc, log...)
}

func (s *Slaver) serve() {
	defer s.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			s.log(logger.Fatal, xpanic.Print(r, "Slaver.serve"))
		}
	}()

	s.logf(logger.Info, "start connect listener (%s %s)", s.lNetwork, s.lAddress)
	defer s.logf(logger.Info, "stop connect listener (%s %s)", s.lNetwork, s.lAddress)

	// dial loop
	for {
		if s.full() {
			s.log(logger.Warning, "full connection")
			select {
			case <-s.sleeper.Sleep(1, 3):
			case <-s.ctx.Done():
				return
			}
			continue
		}
		if s.stopped() {
			return
		}
		conn, err := s.dial()
		if err != nil {
			s.log(logger.Error, err)
			select {
			case <-s.sleeper.Sleep(1, 10):
			case <-s.ctx.Done():
				return
			}
			continue
		}
		s.newConn(conn).Serve()
	}
}

func (s *Slaver) full() bool {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return len(s.conns) >= s.opts.MaxConns
}

func (s *Slaver) stopped() bool {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return !s.start
}

func (s *Slaver) dial() (net.Conn, error) {
	ctx, cancel := context.WithTimeout(s.ctx, s.opts.DialTimeout)
	defer cancel()
	return s.dialer.DialContext(ctx, s.lNetwork, s.lAddress)
}

func (s *Slaver) newConn(c net.Conn) *sConn {
	return &sConn{
		slaver: s,
		local:  c,
	}
}

func (s *Slaver) trackConn(conn *sConn, add bool) bool {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	if add {
		if !s.start {
			return false
		}
		s.conns[conn] = struct{}{}
	} else {
		delete(s.conns, conn)
	}
	return true
}

type sConn struct {
	slaver *Slaver
	local  net.Conn
}

func (c *sConn) log(lv logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = logger.Conn(c.local).WriteTo(buf)
	c.slaver.log(lv, buf)
}

func (c *sConn) Serve() {
	done := make(chan struct{}, 3)
	c.slaver.wg.Add(1)
	go c.serve(done)
	select {
	case <-done:
	case <-c.slaver.ctx.Done():
	}
}

func (c *sConn) serve(done chan<- struct{}) {
	defer c.slaver.wg.Done()

	defer func() {
		// send done signal
		select {
		case done <- struct{}{}:
		case <-c.slaver.ctx.Done():
		}
	}()

	const title = "sConn.serve"
	defer func() {
		if r := recover(); r != nil {
			c.log(logger.Fatal, xpanic.Print(r, title))
			// must wait or make dial storm
			time.Sleep(time.Second)
		}
	}()

	defer func() {
		err := c.local.Close()
		if err != nil && !nettool.IsNetClosingError(err) {
			c.log(logger.Error, "failed to close local connection:", err)
		}
	}()

	if !c.slaver.trackConn(c, true) {
		return
	}
	defer c.slaver.trackConn(c, false)

	// connect the target
	ctx, cancel := context.WithTimeout(c.slaver.ctx, c.slaver.opts.ConnectTimeout)
	defer cancel()
	network := c.slaver.dstNetwork
	address := c.slaver.dstAddress
	remote, err := new(net.Dialer).DialContext(ctx, network, address)
	if err != nil {
		c.log(logger.Error, "failed to connect target:", err)
		return
	}

	defer func() {
		err := remote.Close()
		if err != nil && !nettool.IsNetClosingError(err) {
			c.log(logger.Error, "failed to close remote connection:", err)
		}
	}()

	// log
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, "connect listener and target")
	_, _ = logger.Conn(c.local).WriteTo(buf)
	_, _ = fmt.Fprint(buf, "\n", c.slaver.Status())
	c.slaver.log(logger.Info, buf)

	c.slaver.wg.Add(1)
	go func() {
		defer c.slaver.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				c.log(logger.Fatal, xpanic.Print(r, title))
			}
		}()

		// read one byte for block it, prevent slaver burst connect listener.
		oneByte := make([]byte, 1)
		_ = remote.SetReadDeadline(time.Now().Add(10 * time.Minute))
		_, err := remote.Read(oneByte)
		if err != nil {
			return
		}
		_ = c.local.SetWriteDeadline(time.Now().Add(c.slaver.opts.ConnectTimeout))
		_, err = c.local.Write(oneByte)
		if err != nil {
			c.log(logger.Error, "failed to write to listener connection:", err)
			return
		}

		// send done signal
		select {
		case done <- struct{}{}:
		case <-c.slaver.ctx.Done():
			return
		}

		// continue copy
		_ = remote.SetReadDeadline(time.Time{})
		_ = c.local.SetWriteDeadline(time.Time{})

		_, _ = io.Copy(c.local, remote)
	}()

	// read one byte for block it, prevent slaver burst connect listener.
	oneByte := make([]byte, 1)
	_ = c.local.SetReadDeadline(time.Now().Add(10 * time.Minute))
	_, err = c.local.Read(oneByte)
	if err != nil {
		return
	}
	_ = remote.SetWriteDeadline(time.Now().Add(c.slaver.opts.ConnectTimeout))
	_, err = remote.Write(oneByte)
	if err != nil {
		c.log(logger.Error, "failed to write to remote connection:", err)
		return
	}

	// send done signal
	select {
	case done <- struct{}{}:
	case <-c.slaver.ctx.Done():
		return
	}

	// continue copy
	_ = c.local.SetReadDeadline(time.Time{})
	_ = remote.SetWriteDeadline(time.Time{})

	_, _ = io.Copy(remote, c.local)
}

func (c *sConn) Close() error {
	return c.local.Close()
}

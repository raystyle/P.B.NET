package lcx

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/netutil"

	"project/internal/logger"
	"project/internal/xpanic"
)

// Tranner is used to map port.
type Tranner struct {
	tag          string
	destNetwork  string
	destAddress  *net.TCPAddr
	localNetwork string
	localAddress *net.TCPAddr
	logger       logger.Logger
	opts         *Options

	listener net.Listener
	mutex    sync.Mutex

	wg sync.WaitGroup
}

// NewTranner is used to create a tranner.
func NewTranner(tag, dNetwork, dAddress string, lg logger.Logger, opts *Options) (*Tranner, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}
	destAddr, err := net.ResolveTCPAddr(dNetwork, dAddress)
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = new(Options)
	}
	opts = opts.apply()
	localAddr, err := net.ResolveTCPAddr(opts.LocalNetwork, opts.LocalAddress)
	if err != nil {
		return nil, err
	}
	return &Tranner{
		tag:          "tranner-" + tag,
		destNetwork:  dNetwork,
		destAddress:  destAddr,
		localNetwork: opts.LocalNetwork,
		localAddress: localAddr,
		logger:       lg,
	}, nil
}

// Start is used to start tranner.
func (t *Tranner) Start() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.listener != nil {
		return errors.New("already start tranner")
	}
	listener, err := net.ListenTCP(t.localNetwork, t.localAddress)
	if err != nil {
		return err
	}
	t.listener = netutil.LimitListener(listener, t.opts.MaxConns)
	t.wg.Add(1)
	go t.serve()
	return nil
}

func (t *Tranner) logf(lv logger.Level, format string, log ...interface{}) {
	t.logger.Printf(lv, t.tag, format, log...)
}

func (t *Tranner) log(lv logger.Level, log ...interface{}) {
	t.logger.Println(lv, t.tag, log...)
}

func (t *Tranner) serve() {
	defer func() {
		if r := recover(); r != nil {
			t.log(logger.Fatal, xpanic.Print(r, "Tranner.serve"))
			// restart tran
			time.Sleep(time.Second)
			go t.serve()
		} else {
			t.wg.Done()
		}
		t.logf(logger.Info, "listener closed (%s %s)", t.localNetwork, t.localAddress)
	}()
	t.logf(logger.Info, "start listener (%s %s)", t.localNetwork, t.localAddress)

	// start accept
	var delay time.Duration // how long to sleep on accept failure
	maxDelay := time.Second
	for {
		conn, err := t.listener.Accept()
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
				t.logf(logger.Warning, "accept error: %s; retrying in %v", err, delay)
				time.Sleep(delay)
				continue
			}
			errStr := err.Error()
			if !strings.Contains(errStr, "closed") {
				t.log(logger.Error, errStr)
			}
			return
		}
		delay = 0
		t.wg.Add(1)
		go t.tran(conn)
	}
}

func (t *Tranner) tran(local net.Conn) {
	const title = "Tranner.tran"
	defer func() {
		if r := recover(); r != nil {
			t.log(logger.Fatal, xpanic.Print(r, title))
		}
		t.wg.Done()
	}()
	// connect the target

	remote, err := net.DialTCP(t.destNetwork, nil, t.destAddress)
	if err != nil {
		t.log(logger.Error, "failed to connect target:", err)
		return
	}
	defer func() { _ = remote.Close() }()
	_ = remote.SetDeadline(time.Time{})
	_ = local.SetDeadline(time.Time{})
	t.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.log(logger.Fatal, xpanic.Print(r, title))
			}
			t.wg.Done()
		}()
		_, _ = io.Copy(local, remote)
	}()
	_, _ = io.Copy(remote, local)
}

// Stop is used to stop tranner.
func (t *Tranner) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.listener == nil {
		return
	}
	_ = t.listener.Close()
	t.listener = nil
	t.wg.Wait()
}

// Restart is used to restart tranner.
func (t *Tranner) Restart() error {
	t.Stop()
	return t.Start()
}

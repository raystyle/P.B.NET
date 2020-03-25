package virtualconn

import (
	"context"
	"net"
	"time"

	"github.com/pkg/errors"

	"project/internal/guid"
)

// Listener is a listener to accept virtual connection.
type Listener struct {
	// when source guid equal this, listener can accept it.
	addr *vcAddr

	// virtual connections but not accepted
	conns chan *Conn

	ctx    context.Context
	cancel context.CancelFunc
}

// NewListener is used to create a listener with a timeout.
func NewListener(guid *guid.GUID, port uint32, timeout time.Duration) *Listener {
	listener := Listener{
		conns: make(chan *Conn, 1024),
		addr:  newVCAddr(guid, port),
	}
	if timeout < 1 {
		listener.ctx, listener.cancel = context.WithCancel(context.Background())
	} else {
		listener.ctx, listener.cancel = context.WithTimeout(context.Background(), timeout)
	}
	return &listener
}

// Accept is used to accept a virtual connection.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.ctx.Done():
		return nil, l.ctx.Err()
	}
}

// AcceptVC is used to accept a virtual connection.
func (l *Listener) AcceptVC() (*Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.ctx.Done():
		return nil, l.ctx.Err()
	}
}

// Addr is used to get the address of the listener.
func (l *Listener) Addr() net.Addr {
	return l.addr
}

// Close is used to close listener.
func (l *Listener) Close() error {
	l.cancel()
	return nil
}

// addConn is used to add a virtual connection to listener.
// virtual connection manager will call it.
func (l *Listener) addConn(conn *Conn) error {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case l.conns <- conn:
		return nil
	case <-timer.C: // Accept() block, or so slow.
		return errors.New("add connection to listener timeout")
	case <-l.ctx.Done():
		return l.ctx.Err()
	}
}

package virtualconn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"project/internal/guid"
	"project/internal/random"
)

// vcAddr is the virtual connection address.
type vcAddr struct {
	addr string
}

func (addr *vcAddr) Network() string {
	return "vc" // virtual connection
}

func (addr *vcAddr) String() string {
	return addr.addr
}

func newVCAddr(guid *guid.GUID, port uint32) *vcAddr {
	return &vcAddr{addr: fmt.Sprintf("%s:%d", guid.Hex(), port)}
}

// Sender is from Role sender.
type Sender interface {
	Send(ctx context.Context, data []byte) error
}

// Receiver is from Role handler.
type Receiver interface {
	Receive(ctx context.Context) ([]byte, error)
}

// Conn implement net.Conn.
type Conn struct {
	sender     Sender
	receiver   Receiver
	localAddr  *vcAddr
	remoteAddr *vcAddr

	// received message but not Read()
	recv bytes.Buffer
	rand *random.Rand

	readDeadline  time.Time
	writeDeadline time.Time
	deadlineRWM   sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewConn is used to create a virtual connection.
func NewConn(
	sender Sender,
	receiver Receiver,
	localGUID *guid.GUID,
	localPort uint32,
	remoteGUID *guid.GUID,
	remotePort uint32,
) *Conn {
	conn := Conn{
		sender:     sender,
		receiver:   receiver,
		localAddr:  newVCAddr(localGUID, localPort),
		remoteAddr: newVCAddr(remoteGUID, remotePort),
		rand:       random.New(),
	}
	conn.ctx, conn.cancel = context.WithCancel(context.Background())
	return &conn
}

func (conn *Conn) getReadDeadline() time.Time {
	conn.deadlineRWM.RLock()
	defer conn.deadlineRWM.RUnlock()
	return conn.readDeadline
}

func (conn *Conn) getWriteDeadline() time.Time {
	conn.deadlineRWM.RLock()
	defer conn.deadlineRWM.RUnlock()
	return conn.writeDeadline
}

// Read is used to read data.
func (conn *Conn) Read(b []byte) (int, error) {
	select {
	case <-conn.ctx.Done():
		return 0, io.EOF
	default:
	}
	if len(b) == 0 {
		return 0, nil
	}
	// try to read from buffer, if empty, read receive channel.
	n, err := conn.recv.Read(b)
	if err == nil {
		if conn.recv.Len() == 0 {
			conn.recv.Reset()
		}
		return n, nil
	}
	// set deadline and context
	var ctx context.Context
	deadline := conn.getReadDeadline()
	if !deadline.IsZero() {
		c, cancel := context.WithDeadline(conn.ctx, deadline)
		defer cancel()
		ctx = c
	} else {
		ctx = conn.ctx
	}
	// read new data from receiver
	data, err := conn.receiver.Receive(ctx)
	if err != nil {
		return 0, err
	}
	conn.recv.Write(data)
	return conn.recv.Read(b)
}

// Write is used to write data.
func (conn *Conn) Write(b []byte) (int, error) {
	select {
	case <-conn.ctx.Done():
		return 0, io.EOF
	default:
	}
	// set deadline and context
	var ctx context.Context
	deadline := conn.getWriteDeadline()
	if !deadline.IsZero() {
		c, cancel := context.WithDeadline(conn.ctx, deadline)
		defer cancel()
		ctx = c
	} else {
		ctx = conn.ctx
	}
	// send
	segmentSize := 32*1024 + conn.rand.Int(32*1024)
	var (
		written int
		err     error
	)
	for len(b) > segmentSize {
		err = conn.write(ctx, b[:segmentSize])
		if err != nil {
			return written, err
		}
		written += segmentSize
		b = b[segmentSize:]
	}
	if len(b) == 0 {
		return written, nil
	}
	err = conn.write(ctx, b)
	if err != nil {
		return written, err
	}
	return written + len(b), nil
}

func (conn *Conn) write(ctx context.Context, data []byte) error {
	var (
		err   error
		timer *time.Timer
	)
	for i := 0; i < 3; i++ {
		err = conn.sender.Send(ctx, data)
		if err == nil {
			return nil
		}
		// sleep one second
		if timer == nil {
			timer = time.NewTimer(time.Second)
		} else {
			timer.Reset(time.Second)
		}
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

// LocalAddr is used to get local address.
func (conn *Conn) LocalAddr() net.Addr {
	return conn.localAddr
}

// RemoteAddr is used to get remote address.
func (conn *Conn) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

// SetDeadline is used to set read deadline and write deadline.
func (conn *Conn) SetDeadline(t time.Time) error {
	select {
	case <-conn.ctx.Done():
		return io.EOF
	default:
	}
	conn.deadlineRWM.Lock()
	defer conn.deadlineRWM.Unlock()
	conn.readDeadline = t
	conn.writeDeadline = t
	return nil
}

// SetReadDeadline is used to set read deadline.
func (conn *Conn) SetReadDeadline(t time.Time) error {
	select {
	case <-conn.ctx.Done():
		return io.EOF
	default:
	}
	conn.deadlineRWM.Lock()
	defer conn.deadlineRWM.Unlock()
	conn.readDeadline = t
	return nil
}

// SetWriteDeadline is used to set write deadline.
func (conn *Conn) SetWriteDeadline(t time.Time) error {
	select {
	case <-conn.ctx.Done():
		return io.EOF
	default:
	}
	conn.deadlineRWM.Lock()
	defer conn.deadlineRWM.Unlock()
	conn.writeDeadline = t
	return nil
}

// Close is used to close virtual connection.
func (conn *Conn) Close() error {
	conn.cancel()
	return nil
}

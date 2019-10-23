package xnet

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/xnet/xnetutil"
)

// data size + data
//   uint32      n
const (
	DataSize      = 4
	MaxDataLength = 256 * 1024
)

type Status struct {
	LocalNetwork  string
	LocalAddress  string
	RemoteNetwork string
	RemoteAddress string
	Connect       time.Time
	Send          xnetutil.TrafficUnit
	Receive       xnetutil.TrafficUnit
}

// Conn is used to role handshake, and count network traffic
type Conn struct {
	net.Conn
	connect  time.Time
	sent     int // imprecise
	received int // imprecise
	rwm      sync.RWMutex
}

func NewConn(conn net.Conn, connect time.Time) *Conn {
	return &Conn{
		Conn:    conn,
		connect: connect,
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.rwm.Lock()
	c.received += n
	c.rwm.Unlock()
	return n, err
}

func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.rwm.Lock()
	c.sent += n
	c.rwm.Unlock()
	return n, err
}

// Send is used to send one message
func (c *Conn) Send(msg []byte) error {
	size := convert.Uint32ToBytes(uint32(len(msg)))
	_, err := c.Write(append(size, msg...))
	return err
}

// Receive is used to receive one message
func (c *Conn) Receive() ([]byte, error) {
	sizeBytes := make([]byte, DataSize)
	_, err := io.ReadFull(c, sizeBytes)
	if err != nil {
		return nil, err
	}
	size := int(convert.BytesToUint32(sizeBytes))
	if size > MaxDataLength {
		return nil, errors.Errorf(
			"%s %s receive too big message: %d",
			c.RemoteAddr().Network(), c.RemoteAddr(), size)
	}
	msg := make([]byte, size)
	_, err = io.ReadFull(c, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Conn) Status() *Status {
	c.rwm.RLock()
	s := &Status{
		Send:    xnetutil.TrafficUnit(c.sent),
		Receive: xnetutil.TrafficUnit(c.received),
	}
	c.rwm.RUnlock()
	// the remote address maybe changed, such as QUIC
	s.LocalNetwork = c.LocalAddr().Network()
	s.LocalAddress = c.LocalAddr().String()
	s.RemoteNetwork = c.RemoteAddr().Network()
	s.RemoteAddress = c.RemoteAddr().String()
	s.Connect = c.connect
	return s
}

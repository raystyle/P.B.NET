package xnet

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/xnet/xnetutil"
)

// data size + data
//   uint32      n
const (
	DataSize      = 4
	MaxDataLength = 256 << 10
)

// Status is used show connection status
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

// NewConn is used to wrap a net.Conn to *Conn
func NewConn(conn net.Conn, connect time.Time) *Conn {
	return &Conn{
		Conn:    conn,
		connect: connect,
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.rwm.Lock()
	defer c.rwm.Unlock()
	c.received += n
	return n, err
}

func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.rwm.Lock()
	c.rwm.Unlock()
	c.sent += n
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
		return nil, fmt.Errorf(
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

// Status is used to get connection status
func (c *Conn) Status() *Status {
	c.rwm.RLock()
	defer c.rwm.RUnlock()
	s := &Status{
		Send:    xnetutil.TrafficUnit(c.sent),
		Receive: xnetutil.TrafficUnit(c.received),
	}
	// the remote address maybe changed, such as QUIC
	s.LocalNetwork = c.LocalAddr().Network()
	s.LocalAddress = c.LocalAddr().String()
	s.RemoteNetwork = c.RemoteAddr().Network()
	s.RemoteAddress = c.RemoteAddr().String()
	s.Connect = c.connect
	return s
}

// local tcp 127.0.0.1:123 <-> remote tcp 127.0.0.1:124
// sent: 123 Byte received: 1.101 KB
// connect time: 2006-01-02 15:04:05
func (c *Conn) String() string {
	const format = "local %s %s <-> remote %s %s\n" +
		"sent: %s received: %s\n" +
		"connect time: %s"
	s := c.Status()
	return fmt.Sprintf(format,
		s.LocalNetwork, s.LocalAddress,
		s.RemoteNetwork, s.RemoteAddress,
		s.Send, s.Receive,
		s.Connect.Local().Format(logger.TimeLayout),
	)
}

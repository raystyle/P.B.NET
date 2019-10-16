package xnet

import (
	"io"
	"net"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/logger"
)

// data size + data
//   uint32      n
const (
	DataSize = 4
)

type Status struct {
	LocalNetwork  string
	LocalAddress  string
	RemoteNetwork string
	RemoteAddress string
	Connect       time.Time
	Send          int
	Receive       int
}

// Conn is used to role handshake, and count network traffic
type Conn struct {
	net.Conn
	connect time.Time
	send    int // imprecise
	receive int // imprecise
	rwm     sync.RWMutex
}

func NewConn(conn net.Conn, time time.Time) *Conn {
	return &Conn{
		Conn:    conn,
		connect: time,
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.rwm.Lock()
	c.receive += n
	c.rwm.Unlock()
	return n, err
}

func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.rwm.Lock()
	c.send += n
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
	size := make([]byte, DataSize)
	_, err := io.ReadFull(c, size)
	if err != nil {
		return nil, err
	}
	msg := make([]byte, int(convert.BytesToUint32(size)))
	_, err = io.ReadFull(c, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Conn) String() string {
	return logger.Conn(c).String()
}

func (c *Conn) Status() *Status {
	c.rwm.RLock()
	s := &Status{
		Send:    c.send,
		Receive: c.receive,
	}
	c.rwm.RUnlock()
	// the remote address will change, such as QUIC
	s.LocalNetwork = c.LocalAddr().Network()
	s.LocalAddress = c.LocalAddr().String()
	s.RemoteNetwork = c.RemoteAddr().Network()
	s.RemoteAddress = c.RemoteAddr().String()
	s.Connect = c.connect
	return s
}

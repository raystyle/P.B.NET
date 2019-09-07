package xnet

import (
	"io"
	"net"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/xnet/internal"
)

// header size +  data
//      4          n
const (
	HeaderSize = 4
)

type Info struct {
	LocalNetwork  string
	LocalAddress  string
	RemoteNetwork string
	RemoteAddress string
	ConnectTime   int64
	Send          int
	Receive       int
}

type Conn struct {
	net.Conn
	lNetwork string
	lAddress string
	rNetwork string
	rAddress string
	connect  int64 // timestamp
	send     int   // imprecise
	receive  int   // imprecise
	rwm      sync.RWMutex
}

func NewConn(c net.Conn, now int64) *Conn {
	return &Conn{
		Conn:     c,
		lNetwork: c.LocalAddr().Network(),
		lAddress: c.LocalAddr().String(),
		rNetwork: c.RemoteAddr().Network(),
		rAddress: c.RemoteAddr().String(),
		connect:  now,
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.rwm.Lock()
	c.receive += n
	c.rwm.Unlock()
	if err != nil {
		return n, err
	}
	return n, nil
}

func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.rwm.Lock()
	c.send += n
	c.rwm.Unlock()
	if err != nil {
		return n, err
	}
	return n, nil
}

// send message
func (c *Conn) Send(msg []byte) error {
	size := convert.Uint32ToBytes(uint32(len(msg)))
	_, err := c.Write(append(size, msg...))
	return err
}

// receive message
func (c *Conn) Receive() ([]byte, error) {
	size := make([]byte, HeaderSize)
	_, err := io.ReadFull(c, size)
	if err != nil {
		return nil, err
	}
	s := convert.BytesToUint32(size)
	msg := make([]byte, int(s))
	_, err = io.ReadFull(c, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *Conn) Info() *Info {
	c.rwm.RLock()
	i := &Info{
		Send:    c.send,
		Receive: c.receive,
	}
	c.rwm.RUnlock()
	i.LocalNetwork = c.lNetwork
	i.LocalAddress = c.lAddress
	i.RemoteNetwork = c.rNetwork
	i.RemoteAddress = c.rAddress
	i.ConnectTime = c.connect
	return i
}

func NewDeadlineConn(conn net.Conn, deadline time.Duration) net.Conn {
	return internal.NewDeadlineConn(conn, deadline)
}

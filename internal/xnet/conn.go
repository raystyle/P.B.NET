package xnet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"project/internal/convert"
	"project/internal/logger"
)

// Conn is used to count network traffic and save connect time.
type Conn struct {
	net.Conn

	// imprecise
	sent     uint64
	received uint64
	rwm      sync.RWMutex

	mode    string
	network string // default network
	connect time.Time
}

// NewConn is used to wrap a net.Conn to *Conn.
func NewConn(conn net.Conn, mode string, connect time.Time) *Conn {
	return &Conn{
		Conn:    conn,
		mode:    mode,
		network: defaultNetwork[mode],
		connect: connect.Local(),
	}
}

// Read reads data from the connection.
// It will record network traffic.
func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.rwm.Lock()
	defer c.rwm.Unlock()
	c.received += uint64(n)
	return n, err
}

// Write writes data to the connection.
// It will record network traffic.
func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.rwm.Lock()
	defer c.rwm.Unlock()
	c.sent += uint64(n)
	return n, err
}

// +--------------+---------+
// | size(uint32) | message |
// +--------------+---------+
// |   4 bytes    |   var   |
// +--------------+---------+
const (
	headerSize   = 4          // message size
	MaxMsgLength = 256 * 1024 // 256 KB
)

// errors
var (
	ErrSendTooLargeMessage    = errors.New("send too large message")
	ErrReceiveTooLargeMessage = errors.New("receive too large message")
)

// Send is used to send one message.
func (c *Conn) Send(msg []byte) error {
	size := len(msg)
	if size > MaxMsgLength {
		return ErrSendTooLargeMessage
	}
	header := convert.BEUint32ToBytes(uint32(size))
	_, err := c.Write(append(header, msg...))
	return err
}

// Receive is used to receive one message.
func (c *Conn) Receive() ([]byte, error) {
	header := make([]byte, headerSize)
	_, err := io.ReadFull(c, header)
	if err != nil {
		return nil, err
	}
	msgSize := int(convert.BEBytesToUint32(header))
	if msgSize > MaxMsgLength {
		return nil, ErrReceiveTooLargeMessage
	}
	msg := make([]byte, msgSize)
	_, err = io.ReadFull(c, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// Mode is used to get the connection mode.
func (c *Conn) Mode() string {
	return c.mode
}

// RawConn is used to get the raw conn.
func (c *Conn) RawConn() net.Conn {
	return c.Conn
}

// Status contains connection status.
type Status struct {
	LocalNetwork   string
	LocalAddress   string
	RemoteNetwork  string
	RemoteAddress  string
	Sent           uint64
	Received       uint64
	Mode           string
	DefaultNetwork string
	Connect        time.Time
}

// Status is used to get connection status.
// address maybe changed, such as QUIC.
func (c *Conn) Status() *Status {
	c.rwm.RLock()
	defer c.rwm.RUnlock()
	return &Status{
		LocalNetwork:   c.LocalAddr().Network(),
		LocalAddress:   c.LocalAddr().String(),
		RemoteNetwork:  c.RemoteAddr().Network(),
		RemoteAddress:  c.RemoteAddr().String(),
		Sent:           c.sent,
		Received:       c.received,
		Mode:           c.mode,
		DefaultNetwork: c.network,
		Connect:        c.connect,
	}
}

// String is used to get connection information.
//
// local:  tcp 127.0.0.1:123
// remote: tcp 127.0.0.1:124
// sent:   123 Byte, received: 1.101 KB
// mode:   tls,  default network: tcp
// connect time: 2018-11-27 00:00:00
func (c *Conn) String() string {
	const format = "" +
		"local:  %s %s\n" +
		"remote: %s %s\n" +
		"sent:   %s, received: %s\n" +
		"mode:   %-5s default network: %s\n" +
		"connect time: %s"
	s := c.Status()
	return fmt.Sprintf(format,
		s.LocalNetwork, s.LocalAddress,
		s.RemoteNetwork, s.RemoteAddress,
		convert.FormatByte(s.Sent),
		convert.FormatByte(s.Received),
		s.Mode+",", s.DefaultNetwork,
		s.Connect.Format(logger.TimeLayout),
	)
}

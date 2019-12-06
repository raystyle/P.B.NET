package node

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

const (
	connUsageServeCtrl = iota
	connUsageServeNode
	connUsageServeBeacon
	connUsageClient
)

type conn struct {
	logger logger.Logger
	conn   *xnet.Conn
	usage  int

	logSrc    string
	slots     []*protocol.Slot
	heartbeat chan struct{}

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
}

func newConn(lg logger.Logger, xc *xnet.Conn, usage int) *conn {
	conn := conn{
		logger:     lg,
		conn:       xc,
		stopSignal: make(chan struct{}),
	}
	_ = xc.SetDeadline(time.Time{})

	conn.slots = make([]*protocol.Slot, protocol.SlotSize)
	for i := 0; i < protocol.SlotSize; i++ {
		slot := &protocol.Slot{
			Available: make(chan struct{}, 1),
			Reply:     make(chan []byte, 1),
			Timer:     time.NewTimer(protocol.RecvTimeout),
		}
		slot.Available <- struct{}{}
		conn.slots[i] = slot
	}

	switch usage {
	case connUsageServeCtrl:
		conn.logSrc = "ctrl-conn"
	case connUsageServeNode:
		conn.logSrc = "node-conn"
	case connUsageServeBeacon:
		conn.logSrc = "beacon-conn"
	case connUsageClient:
		conn.logSrc = "client-conn"
	default:
		panic(fmt.Sprintf("invalid conn usage: %d", usage))
	}
	return &conn
}

func (c *conn) log(l logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprint(b, log...)
	_, _ = fmt.Fprintf(b, "\n%s", c.conn)
	c.logger.Print(l, c.logSrc, b)
}

func (c *conn) isClosed() bool {
	return atomic.LoadInt32(&c.inClose) != 0
}

// msg id(2 bytes) + data
func (c *conn) handleReply(reply []byte) {
	l := len(reply)
	if l < protocol.MsgIDSize {
		c.log(logger.Exploit, protocol.ErrRecvInvalidMsgIDSize)
		c.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:protocol.MsgIDSize]))
	if id > protocol.MaxMsgID {
		c.log(logger.Exploit, protocol.ErrRecvInvalidMsgID)
		c.Close()
		return
	}
	// must copy
	r := make([]byte, l-protocol.MsgIDSize)
	copy(r, reply[protocol.MsgIDSize:])
	// <security> maybe incorrect msg id
	select {
	case c.slots[id].Reply <- r:
	default:
		c.log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		c.Close()
	}
}

func (c *conn) onFrame(frame []byte) bool {
	if c.isClosed() {
		return true
	}
	// cmd(1) + msg id(2)
	if len(frame) < protocol.MsgCMDSize+protocol.MsgIDSize {
		c.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		c.Close()
		return true
	}
	switch frame[0] {
	case protocol.ConnReply:
		c.handleReply(frame[protocol.MsgCMDSize:])
		return true
	case protocol.ErrCMDRecvNullMsg:
		c.log(logger.Exploit, protocol.ErrRecvNullMsg)
		c.Close()
		return true
	case protocol.ErrCMDTooBigMsg:
		c.log(logger.Exploit, protocol.ErrRecvTooBigMsg)
		c.Close()
		return true
	case protocol.TestCommand:
		id := frame[protocol.MsgCMDSize : protocol.MsgCMDSize+protocol.MsgIDSize]
		data := frame[protocol.MsgCMDSize+protocol.MsgIDSize:]
		c.Reply(id, data)
		return true
	}
	return false
}

// Reply is used to reply command
func (c *conn) Reply(id, reply []byte) {
	if c.isClosed() {
		return
	}
	l := len(reply)
	// 7 = size(4 Bytes) + NodeReply(1 byte) + msg id(2 bytes)
	b := make([]byte, protocol.MsgHeaderSize+l)
	// write size
	msgSize := protocol.MsgCMDSize + protocol.MsgIDSize + l
	copy(b, convert.Uint32ToBytes(uint32(msgSize)))
	// write cmd
	b[protocol.MsgLenSize] = protocol.ConnReply
	// write msg id
	copy(b[protocol.MsgLenSize+1:protocol.MsgLenSize+1+protocol.MsgIDSize], id)
	// write data
	copy(b[protocol.MsgHeaderSize:], reply)
	_ = c.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = c.conn.Write(b)
}

// Send is used to send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (c *conn) Send(cmd uint8, data []byte) ([]byte, error) {
	if c.isClosed() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-c.slots[id].Available:
				l := len(data)
				b := make([]byte, protocol.MsgHeaderSize+l)
				// write MsgLen
				msgSize := protocol.MsgCMDSize + protocol.MsgIDSize + l
				copy(b, convert.Uint32ToBytes(uint32(msgSize)))
				// write cmd
				b[protocol.MsgLenSize] = cmd
				// write msg id
				copy(b[protocol.MsgLenSize+1:protocol.MsgLenSize+1+protocol.MsgIDSize],
					convert.Uint16ToBytes(uint16(id)))
				// write data
				copy(b[protocol.MsgHeaderSize:], data)
				// send
				_ = c.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := c.conn.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				if !c.slots[id].Timer.Stop() {
					<-c.slots[id].Timer.C
				}
				c.slots[id].Timer.Reset(protocol.RecvTimeout)
				select {
				case r := <-c.slots[id].Reply:
					c.slots[id].Available <- struct{}{}
					return r, nil
				case <-c.slots[id].Timer.C:
					c.Close()
					return nil, protocol.ErrRecvTimeout
				case <-c.stopSignal:
					return nil, protocol.ErrConnClosed
				}
			case <-c.stopSignal:
				return nil, protocol.ErrConnClosed
			default:
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-c.stopSignal:
			return nil, protocol.ErrConnClosed
		}
	}
}

func (c *conn) Status() *xnet.Status {
	return c.conn.Status()
}

func (c *conn) Close() {
	c.closeOnce.Do(func() {
		atomic.StoreInt32(&c.inClose, 1)
		close(c.stopSignal)
		_ = c.conn.Close()
	})
}

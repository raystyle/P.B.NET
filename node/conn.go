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
	*xnet.Conn

	logSrc string
	slots  []*protocol.Slot

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
}

func newConn(lg logger.Logger, xConn *xnet.Conn, usage int) *conn {
	conn := conn{
		logger:     lg,
		Conn:       xConn,
		stopSignal: make(chan struct{}),
	}
	_ = xConn.SetDeadline(time.Time{})
	conn.slots = make([]*protocol.Slot, protocol.SlotSize)
	for i := 0; i < protocol.SlotSize; i++ {
		conn.slots[i] = protocol.NewSlot()
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
	_, _ = fmt.Fprintf(b, "\n%s", c)
	c.logger.Print(l, c.logSrc, b)
}

func (c *conn) isClosed() bool {
	return atomic.LoadInt32(&c.inClose) != 0
}

func (c *conn) onFrame(frame []byte) bool {
	if c.isClosed() {
		return true
	}
	// cmd(1) + msg id(2)
	if len(frame) < protocol.FrameCMDSize+protocol.FrameIDSize {
		c.log(logger.Exploit, protocol.ErrInvalidFrameSize)
		_ = c.Close()
		return true
	}
	switch frame[0] {
	case protocol.ConnReply:
		c.handleReply(frame[protocol.FrameCMDSize:])
	case protocol.ConnErrRecvNullFrame:
		c.log(logger.Exploit, protocol.ErrRecvNullFrame)
		_ = c.Close()
	case protocol.ConnErrRecvTooBigFrame:
		c.log(logger.Exploit, protocol.ErrRecvTooBigFrame)
		_ = c.Close()
	case protocol.TestCommand:
		id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
		data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
		c.Reply(id, data)
	default:
		return false
	}
	return true
}

// msg id(2 bytes) + data
func (c *conn) handleReply(reply []byte) {
	l := len(reply)
	if l < protocol.FrameIDSize {
		c.log(logger.Exploit, protocol.ErrRecvInvalidFrameIDSize)
		_ = c.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:protocol.FrameIDSize]))
	if id > protocol.MaxFrameID {
		c.log(logger.Exploit, protocol.ErrRecvInvalidFrameID)
		_ = c.Close()
		return
	}
	// must copy
	r := make([]byte, l-protocol.FrameIDSize)
	copy(r, reply[protocol.FrameIDSize:])
	// <security> maybe incorrect msg id
	select {
	case c.slots[id].Reply <- r:
	default:
		c.log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		_ = c.Close()
	}
}

// Reply is used to reply command
func (c *conn) Reply(id, reply []byte) {
	if c.isClosed() {
		return
	}
	l := len(reply)
	// 7 = size(4 Bytes) + NodeReply(1 byte) + msg id(2 bytes)
	b := make([]byte, protocol.FrameHeaderSize+l)
	// write size
	msgSize := protocol.FrameCMDSize + protocol.FrameIDSize + l
	copy(b, convert.Uint32ToBytes(uint32(msgSize)))
	// write cmd
	b[protocol.FrameLenSize] = protocol.ConnReply
	// write msg id
	copy(b[protocol.FrameLenSize+1:protocol.FrameLenSize+1+protocol.FrameIDSize], id)
	// write data
	copy(b[protocol.FrameHeaderSize:], reply)
	_ = c.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = c.Write(b)
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
				b := make([]byte, protocol.FrameHeaderSize+l)
				// write MsgLen
				msgSize := protocol.FrameCMDSize + protocol.FrameIDSize + l
				copy(b, convert.Uint32ToBytes(uint32(msgSize)))
				// write cmd
				b[protocol.FrameLenSize] = cmd
				// write msg id
				copy(b[protocol.FrameLenSize+1:protocol.FrameLenSize+1+protocol.FrameIDSize],
					convert.Uint16ToBytes(uint16(id)))
				// write data
				copy(b[protocol.FrameHeaderSize:], data)
				// send
				_ = c.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := c.Write(b)
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
					_ = c.Close()
					return nil, protocol.ErrRecvReplyTimeout
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

func (c *conn) SendRaw(b []byte) error {
	return c.Conn.Send(b)
}

func (c *conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		atomic.StoreInt32(&c.inClose, 1)
		close(c.stopSignal)
		err = c.Conn.Close()
	})
	return err
}

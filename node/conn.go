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
	"project/internal/xpanic"
)

const (
	connUsageServeCtrl = iota
	connUsageServeNode
	connUsageServeBeacon
	connUsageClient
)

type conn struct {
	ctx *Node

	conn  *xnet.Conn
	usage int
	guid  []byte

	logSrc    string
	slots     []*protocol.Slot
	heartbeat chan struct{}
	sync      int32

	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newConn(ctx *Node, xc *xnet.Conn, usage int, guid []byte) *conn {
	conn := conn{
		ctx:   ctx,
		conn:  xc,
		usage: usage,
		guid:  guid,
	}

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

	conn.stopSignal = make(chan struct{})

	switch usage {
	case connUsageServeCtrl:
		conn.logSrc = "ctrl-conn"
	case connUsageServeNode:
		conn.logSrc = "node-conn"
	case connUsageServeBeacon:
		conn.logSrc = "beacon-conn"
	case connUsageClient:
		conn.logSrc = "client"
		conn.heartbeat = make(chan struct{}, 1)
		conn.wg.Add(1)
		go conn.sendHeartbeat()
	default:
		panic(fmt.Sprintf("invalid conn usage: %d", usage))
	}

	// <warning> not add wg
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := xpanic.Error(r, conn.logSrc)
				conn.log(logger.Fatal, err)
			}
			conn.Close()
		}()
		switch usage {
		case connUsageServeCtrl:
			protocol.HandleConn(conn.conn, conn.onFrameServeCtrl)
		case connUsageServeNode:
			protocol.HandleConn(conn.conn, conn.onFrameServeNode)
		case connUsageServeBeacon:
			protocol.HandleConn(conn.conn, conn.onFrameServeBeacon)
		case connUsageClient:
			protocol.HandleConn(conn.conn, conn.onFrameClient)
		}
	}()
	return &conn
}

func (c *conn) logf(l logger.Level, format string, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, format, log...)
	_, _ = fmt.Fprintf(b, "\n%s", c.conn)
	c.ctx.logger.Print(l, c.logSrc, b)
}

func (c *conn) log(l logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprint(b, log...)
	_, _ = fmt.Fprintf(b, "\n%s", c.conn)
	c.ctx.logger.Print(l, c.logSrc, b)
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
	// <security> maybe wrong msg id
	select {
	case c.slots[id].Reply <- r:
	default:
		c.log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		c.Close()
	}
}

func (c *conn) onFrame(frame []byte) bool {
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if c.isClosed() {
		return false
	}
	// cmd(1) + msg id(2) or reply
	if len(frame) < id {
		c.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		c.Close()
		return false
	}
	// check command
	switch frame[0] {
	case protocol.ConnReply:
		c.handleReply(frame[cmd:])
	case protocol.ErrCMDRecvNullMsg:
		c.log(logger.Exploit, protocol.ErrRecvNullMsg)
		c.Close()
		return false
	case protocol.ErrCMDTooBigMsg:
		c.log(logger.Exploit, protocol.ErrRecvTooBigMsg)
		c.Close()
		return false
	case protocol.TestCommand:
		c.Reply(frame[cmd:id], frame[id:])
	}
	return true
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

func (c *conn) Close() {
	c.closeOnce.Do(func() {
		atomic.StoreInt32(&c.inClose, 1)
		close(c.stopSignal)
		_ = c.conn.Close()
		c.wg.Wait()
		// <security> can't record controller
		// client don't need record
		switch c.usage {
		case connUsageServeNode:
		case connUsageServeBeacon:
		default:
			return
		}
		c.log(logger.Info, "disconnected")
	})
}

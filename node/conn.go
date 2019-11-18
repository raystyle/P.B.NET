package node

import (
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

func (conn *conn) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(conn.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	conn.ctx.logger.Print(l, conn.logSrc, b)
}

func (conn *conn) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(conn.conn)
	_, _ = fmt.Fprint(b, log...)
	conn.ctx.logger.Print(l, conn.logSrc, b)
}

func (conn *conn) isClosed() bool {
	return atomic.LoadInt32(&conn.inClose) != 0
}

func (conn *conn) Close() {
	conn.closeOnce.Do(func() {
		atomic.StoreInt32(&conn.inClose, 1)
		close(conn.stopSignal)
		_ = conn.conn.Close()
		conn.wg.Wait()

		// <security> can't record controller
		// client don't need record
		switch conn.usage {
		case connUsageServeNode:
		case connUsageServeBeacon:
		default:
			return
		}
		conn.log(logger.Info, "disconnected")
	})
}

// msg id(2 bytes) + data
func (conn *conn) handleReply(reply []byte) {
	l := len(reply)
	if l < protocol.MsgIDSize {
		conn.log(logger.Exploit, protocol.ErrRecvInvalidMsgIDSize)
		conn.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:protocol.MsgIDSize]))
	if id > protocol.MaxMsgID {
		conn.log(logger.Exploit, protocol.ErrRecvInvalidMsgID)
		conn.Close()
		return
	}
	// must copy
	r := make([]byte, l-protocol.MsgIDSize)
	copy(r, reply[protocol.MsgIDSize:])
	// <security> maybe wrong msg id
	select {
	case conn.slots[id].Reply <- r:
	default:
		conn.log(logger.Exploit, protocol.ErrRecvInvalidReplyID)
		conn.Close()
	}
}

func (conn *conn) onFrame(frame []byte) bool {
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if conn.isClosed() {
		return false
	}
	// cmd(1) + msg id(2) or reply
	if len(frame) < id {
		conn.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		conn.Close()
		return false
	}
	// check command
	switch frame[0] {
	case protocol.ConnReply:
		conn.handleReply(frame[cmd:])
	case protocol.ErrCMDRecvNullMsg:
		conn.log(logger.Exploit, protocol.ErrRecvNullMsg)
		conn.Close()
		return false
	case protocol.ErrCMDTooBigMsg:
		conn.log(logger.Exploit, protocol.ErrRecvTooBigMsg)
		conn.Close()
		return false
	case protocol.TestCommand:
		conn.Reply(frame[cmd:id], frame[id:])
	}
	return true
}

// Reply is used to reply command
func (conn *conn) Reply(id, reply []byte) {
	if conn.isClosed() {
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
	_ = conn.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = conn.conn.Write(b)
}

// Send is used to send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (conn *conn) Send(cmd uint8, data []byte) ([]byte, error) {
	if conn.isClosed() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-conn.slots[id].Available:
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
				_ = conn.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := conn.conn.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				if !conn.slots[id].Timer.Stop() {
					<-conn.slots[id].Timer.C
				}
				conn.slots[id].Timer.Reset(protocol.RecvTimeout)
				select {
				case r := <-conn.slots[id].Reply:
					conn.slots[id].Available <- struct{}{}
					return r, nil
				case <-conn.slots[id].Timer.C:
					conn.Close()
					return nil, protocol.ErrRecvTimeout
				case <-conn.stopSignal:
					return nil, protocol.ErrConnClosed
				}
			case <-conn.stopSignal:
				return nil, protocol.ErrConnClosed
			default:
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-conn.stopSignal:
			return nil, protocol.ErrConnClosed
		}
	}
}

package controller

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
)

// NodeGUID != nil for sync or other
// NodeGUID = nil for trust node
// NodeGUID = controller guid for discovery
type clientCfg struct {
	Node     *bootstrap.Node
	NodeGUID []byte
	CloseLog bool
	xnet.Config
}

type client struct {
	ctx        *CTRL
	node       *bootstrap.Node
	guid       []byte
	closeLog   bool
	conn       *xnet.Conn
	slots      []*protocol.Slot
	replyTimer *time.Timer
	inClose    int32
	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newClient(ctx *CTRL, cfg *clientCfg) (*client, error) {
	cfg.Network = cfg.Node.Network
	cfg.Address = cfg.Node.Address
	// TODO add ca cert
	conn, err := xnet.Dial(cfg.Node.Mode, &cfg.Config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	client := client{
		ctx:      ctx,
		node:     cfg.Node,
		guid:     cfg.NodeGUID,
		closeLog: cfg.CloseLog,
	}
	xconn, err := client.handshake(conn)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessage(err, "handshake failed")
	}
	client.conn = xconn
	// init slot
	client.slots = make([]*protocol.Slot, protocol.SlotSize)
	for i := 0; i < protocol.SlotSize; i++ {
		s := &protocol.Slot{
			Available: make(chan struct{}, 1),
			Reply:     make(chan []byte, 1),
			Timer:     time.NewTimer(protocol.RecvTimeout),
		}
		s.Available <- struct{}{}
		client.slots[i] = s
	}
	client.replyTimer = time.NewTimer(time.Second)
	client.stopSignal = make(chan struct{})
	go func() {
		// not add wg, because client.Close
		// TODO recover
		defer func() {
			client.Close()
		}()
		protocol.HandleConn(client.conn, client.handleMessage)
	}()
	client.wg.Add(1)
	go client.heartbeat()
	return &client, nil
}

func (client *client) Close() {
	client.closeOnce.Do(func() {
		atomic.StoreInt32(&client.inClose, 1)
		close(client.stopSignal)
		_ = client.conn.Close()
		client.wg.Wait()
		if client.closeLog {
			client.logln(logger.INFO, "disconnected")
		}
	})
}

func (client *client) isClosed() bool {
	return atomic.LoadInt32(&client.inClose) != 0
}

func (client *client) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(client.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	client.ctx.Print(l, "client", b)
}

func (client *client) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(client.conn)
	_, _ = fmt.Fprint(b, log...)
	client.ctx.Print(l, "client", b)
}

func (client *client) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(client.conn)
	_, _ = fmt.Fprintln(b, log...)
	client.ctx.Print(l, "client", b)
}

// can use this.Close()
func (client *client) handleMessage(msg []byte) {
	if client.isClosed() {
		return
	}
	if len(msg) < 1 {
		client.log(logger.EXPLOIT, protocol.ErrInvalidMsgSize)
		client.Close()
		return
	}
	switch msg[0] {
	case protocol.NodeReply:
		client.handleReply(msg[1:])
	case protocol.NodeHeartbeat: // discard
	case protocol.ErrNullMsg:
		client.log(logger.EXPLOIT, protocol.ErrRecvNullMsg)
		client.Close()
	case protocol.ErrTooBigMsg:
		client.log(logger.EXPLOIT, protocol.ErrRecvTooBigMsg)
		client.Close()
	case protocol.TestMessage:
		if len(msg) < 3 {
			client.log(logger.EXPLOIT, protocol.ErrRecvInvalidTestMsg)
		}
		client.reply(msg[1:3], msg[3:])
	default:
		client.log(logger.EXPLOIT, protocol.ErrRecvUnknownCMD, msg[1:])
		client.Close()
		return
	}
}

func (client *client) heartbeat() {
	defer client.wg.Done()
	rand := random.New(0)
	buffer := bytes.NewBuffer(nil)
	for {
		select {
		case <-time.After(time.Duration(30+rand.Int(60)) * time.Second):
			// <security> fake flow like client
			fakeSize := 64 + rand.Int(256)
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			buffer.Reset()
			buffer.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
			buffer.WriteByte(protocol.CtrlHeartbeat)
			buffer.Write(rand.Bytes(fakeSize))
			// send
			_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
			_, err := client.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
		case <-client.stopSignal:
			return
		}
	}
}

func (client *client) reply(id, reply []byte) {
	if client.isClosed() {
		return
	}
	// size(4 Bytes) + CtrlReply(1 byte) + msg_id(2 bytes)
	l := len(reply)
	b := make([]byte, 7+l)
	copy(b, convert.Uint32ToBytes(uint32(3+l))) // write size
	b[4] = protocol.CtrlReply
	copy(b[5:7], id)
	copy(b[7:], reply)
	_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = client.conn.Write(b)
}

// msg_id(2 bytes) + data
func (client *client) handleReply(reply []byte) {
	l := len(reply)
	if l < 2 {
		client.log(logger.EXPLOIT, protocol.ErrRecvInvalidMsgIDSize)
		client.Close()
		return
	}
	id := int(convert.BytesToUint16(reply[:2]))
	if id > protocol.MaxMsgID {
		client.log(logger.EXPLOIT, protocol.ErrRecvInvalidMsgID)
		client.Close()
		return
	}
	// must copy
	r := make([]byte, l-2)
	copy(r, reply[2:])
	// <security> maybe wrong msg id
	client.replyTimer.Reset(time.Second)
	select {
	case client.slots[id].Reply <- r:
		client.replyTimer.Stop()
	case <-client.replyTimer.C:
		client.log(logger.EXPLOIT, protocol.ErrRecvInvalidReply)
		client.Close()
	}
}

// send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (client *client) Send(cmd uint8, data []byte) ([]byte, error) {
	if client.isClosed() {
		return nil, protocol.ErrConnClosed
	}
	for {
		for id := 0; id < protocol.SlotSize; id++ {
			select {
			case <-client.slots[id].Available:
				l := len(data)
				b := make([]byte, 7+l)
				copy(b, convert.Uint32ToBytes(uint32(3+l))) // write size
				b[4] = cmd
				copy(b[5:7], convert.Uint16ToBytes(uint16(id)))
				copy(b[7:], data)
				// send
				_ = client.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
				_, err := client.conn.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				client.slots[id].Timer.Reset(protocol.RecvTimeout)
				select {
				case r := <-client.slots[id].Reply:
					client.slots[id].Timer.Stop()
					client.slots[id].Available <- struct{}{}
					return r, nil
				case <-client.slots[id].Timer.C:
					client.Close()
					return nil, protocol.ErrRecvTimeout
				case <-client.stopSignal:
					return nil, protocol.ErrConnClosed
				}
			case <-client.stopSignal:
				return nil, protocol.ErrConnClosed
			default:
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-client.stopSignal:
			return nil, protocol.ErrConnClosed
		}
	}
}

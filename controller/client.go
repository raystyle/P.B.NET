package controller

import (
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

// Node_GUID != nil for sync or other
// Node_GUID = nil for trust node
// Node_GUID = controller guid for discovery
type client_cfg struct {
	Node      *bootstrap.Node
	Node_GUID []byte
	Close_Log bool
	xnet.Config
}

type client struct {
	ctx         *CTRL
	node        *bootstrap.Node
	guid        []byte
	close_log   bool
	conn        *xnet.Conn
	send_queue  chan []byte
	slots       []*protocol.Slot
	reply_timer *time.Timer
	in_close    int32
	close_once  sync.Once
	stop_signal chan struct{}
	wg          sync.WaitGroup
}

func new_client(ctx *CTRL, cfg *client_cfg) (*client, error) {
	cfg.Network = cfg.Node.Network
	cfg.Address = cfg.Node.Address
	// TODO add ca cert
	conn, err := xnet.Dial(cfg.Node.Mode, &cfg.Config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c := &client{
		ctx:       ctx,
		node:      cfg.Node,
		guid:      cfg.Node_GUID,
		close_log: cfg.Close_Log,
	}
	err_chan := make(chan error, 1)
	go func() {
		// TODO recover
		xconn, err := c.handshake(conn)
		if err != nil {
			err_chan <- err
			return
		}
		c.conn = xconn
		close(err_chan)
	}()
	select {
	case err = <-err_chan:
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
	case <-time.After(time.Minute):
		_ = conn.Close()
		return nil, errors.New("handshake timeout")
	}
	// init send_queue & slot
	c.send_queue = make(chan []byte, 4*protocol.SLOT_SIZE)
	c.slots = make([]*protocol.Slot, protocol.SLOT_SIZE)
	for i := 0; i < protocol.SLOT_SIZE; i++ {
		s := &protocol.Slot{
			Available: make(chan struct{}, 1),
			Reply:     make(chan []byte, 1),
			Timer:     time.NewTimer(protocol.RECV_TIMEOUT),
		}
		s.Available <- struct{}{}
		c.slots[i] = s
	}
	c.reply_timer = time.NewTimer(time.Second)
	c.stop_signal = make(chan struct{})
	go protocol.Handle_Conn(c.conn, c.handle_message, c.Close)
	c.wg.Add(2)
	go c.sender()
	go c.heartbeat()
	return c, nil
}

func (this *client) Close() {
	this.close_once.Do(func() {
		atomic.StoreInt32(&this.in_close, 1)
		close(this.stop_signal)
		_ = this.conn.Close()
		this.wg.Wait()
		if this.close_log {
			this.logln(logger.INFO, "disconnected")
		}
	})
}

func (this *client) is_closed() bool {
	return atomic.LoadInt32(&this.in_close) != 0
}

func (this *client) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(this.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	this.ctx.Print(l, "client", b)
}

func (this *client) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(this.conn)
	_, _ = fmt.Fprint(b, log...)
	this.ctx.Print(l, "client", b)
}

func (this *client) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(this.conn)
	_, _ = fmt.Fprintln(b, log...)
	this.ctx.Print(l, "client", b)
}

// can use this.Close()
func (this *client) handle_message(msg []byte) {
	if this.is_closed() {
		return
	}
	if len(msg) < 1 {
		this.log(logger.EXPLOIT, protocol.ERR_INVALID_MSG_SIZE)
		this.Close()
		return
	}
	switch msg[0] {
	case protocol.NODE_REPLY:
		this.handle_reply(msg[1:])
	case protocol.NODE_HEARTBEAT: // discard
	case protocol.ERR_NULL_MSG:
		this.log(logger.EXPLOIT, protocol.ERR_RECV_NULL_MSG)
		this.Close()
	case protocol.ERR_TOO_BIG_MSG:
		this.log(logger.EXPLOIT, protocol.ERR_RECV_TOO_BIG_MSG)
		this.Close()
	case protocol.TEST_MSG:
		if len(msg) < 3 {
			this.log(logger.EXPLOIT, protocol.ERR_RECV_INVALID_TEST_MSG)
		}
		this.reply(msg[1:3], msg[3:])
	default:
		this.log(logger.EXPLOIT, protocol.ERR_RECV_UNKNOWN_CMD, msg[1:])
		this.Close()
		return
	}
}

func (this *client) sender() {
	defer this.wg.Done()
	var msg []byte
	for {
		select {
		case msg = <-this.send_queue:
			_ = this.conn.SetWriteDeadline(time.Now().Add(protocol.SEND_TIMEOUT))
			_, err := this.conn.Write(msg)
			if err != nil {
				return
			}
		case <-this.stop_signal:
			return
		}
	}
}

func (this *client) heartbeat() {
	defer this.wg.Done()
	rand := random.New()
	for {
		select {
		case <-time.After(time.Duration(30+rand.Int(60)) * time.Second):
			// <security> fake flow like client
			fake_size := 64 + rand.Int(256)
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			b := make([]byte, 5+fake_size)
			copy(b, convert.Uint32_Bytes(uint32(1+fake_size)))
			b[4] = protocol.CTRL_HEARTBEAT
			copy(b[5:], rand.Bytes(fake_size))
			// send
			select {
			case this.send_queue <- b:
			case <-time.After(protocol.SEND_TIMEOUT):
				this.log(logger.WARNING, protocol.ERR_SEND_TIMEOUT)
				_ = this.conn.Close()
				return
			}
		case <-this.stop_signal:
			return
		}
	}
}

func (this *client) reply(id, reply []byte) {
	if this.is_closed() {
		return
	}
	// size(4 Bytes) + CTRL_REPLY(1 byte) + msg_id(2 bytes)
	l := len(reply)
	b := make([]byte, 7+l)
	copy(b, convert.Uint32_Bytes(uint32(3+l))) // write size
	b[4] = protocol.CTRL_REPLY
	copy(b[5:7], id)
	copy(b[7:], reply)
	select {
	case this.send_queue <- b:
	case <-time.After(protocol.SEND_TIMEOUT):
		this.log(logger.WARNING, protocol.ERR_SEND_TIMEOUT)
		this.Close()
	}
}

// msg_id(2 bytes) + data
func (this *client) handle_reply(reply []byte) {
	l := len(reply)
	if l < 2 {
		this.log(logger.EXPLOIT, protocol.ERR_RECV_INVALID_MSG_ID_SIZE)
		this.Close()
		return
	}
	id := int(convert.Bytes_Uint16(reply[:2]))
	if id > protocol.MAX_MSG_ID {
		this.log(logger.EXPLOIT, protocol.ERR_RECV_INVALID_MSG_ID)
		this.Close()
		return
	}
	// must copy
	r := make([]byte, l-2)
	copy(r, reply[2:])
	// <security> maybe wrong msg id
	this.reply_timer.Reset(time.Second)
	select {
	case this.slots[id].Reply <- r:
	case <-this.reply_timer.C:
		this.log(logger.EXPLOIT, protocol.ERR_RECV_INVALID_REPLY)
		this.Close()
	}
}

// send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (this *client) Send(cmd uint8, data []byte) ([]byte, error) {
	if this.is_closed() {
		return nil, protocol.ERR_CONN_CLOSED
	}
	for {
		for id := 0; id < protocol.SLOT_SIZE; id++ {
			select {
			case <-this.slots[id].Available:
				l := len(data)
				b := make([]byte, 7+l)
				copy(b, convert.Uint32_Bytes(uint32(3+l))) // write size
				b[4] = cmd
				copy(b[5:7], convert.Uint16_Bytes(uint16(id)))
				copy(b[7:], data)
				// send
				select {
				case this.send_queue <- b:
				case <-time.After(protocol.SEND_TIMEOUT):
					this.log(logger.WARNING, protocol.ERR_SEND_TIMEOUT)
					this.Close()
					return nil, protocol.ERR_SEND_TIMEOUT
				}
				// wait for reply
				this.slots[id].Timer.Reset(protocol.RECV_TIMEOUT)
				select {
				case r := <-this.slots[id].Reply:
					this.slots[id].Available <- struct{}{}
					return r, nil
				case <-this.slots[id].Timer.C:
					this.Close()
					return nil, protocol.ERR_RECV_TIMEOUT
				case <-this.stop_signal:
					return nil, protocol.ERR_CONN_CLOSED
				}
			case <-this.stop_signal:
				return nil, protocol.ERR_CONN_CLOSED
			default:
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-this.stop_signal:
			return nil, protocol.ERR_CONN_CLOSED
		}
	}
}

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
	"project/internal/random"
	"project/internal/xnet"
)

// controller client
type c_ctrl struct {
	ctx         *NODE
	conn        *xnet.Conn
	slots       []*protocol.Slot
	reply_timer *time.Timer
	random      *random.Generator // for handle_heartbeat()
	buffer      bytes.Buffer      // for handle_heartbeat()
	in_close    int32
	close_once  sync.Once
	stop_signal chan struct{}
	wg          sync.WaitGroup
}

func (this *server) serve_ctrl(conn *xnet.Conn) {
	c := &c_ctrl{
		ctx:         this.ctx,
		conn:        conn,
		slots:       make([]*protocol.Slot, protocol.SLOT_SIZE),
		reply_timer: time.NewTimer(time.Second),
		random:      random.New(),
		stop_signal: make(chan struct{}),
	}
	this.add_ctrl(c)
	this.log(logger.DEBUG, &s_log{c: conn, l: "controller connected"})
	defer func() {
		this.del_ctrl("", c)
		this.log(logger.DEBUG, &s_log{c: conn, l: "controller disconnected"})
	}()
	// init slot
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
	protocol.Handle_Conn(conn, c.handle_message, c.Close)
}

func (this *c_ctrl) Info() *xnet.Info {
	return this.conn.Info()
}

func (this *c_ctrl) Close() {
	this.close_once.Do(func() {
		atomic.StoreInt32(&this.in_close, 1)
		close(this.stop_signal)
		_ = this.conn.Close()
		this.wg.Wait()
	})
}

func (this *c_ctrl) is_closed() bool {
	return atomic.LoadInt32(&this.in_close) != 0
}

func (this *c_ctrl) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(this.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	this.ctx.Print(l, "c_ctrl", b)
}

func (this *c_ctrl) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(this.conn)
	_, _ = fmt.Fprint(b, log...)
	this.ctx.Print(l, "c_ctrl", b)
}

func (this *c_ctrl) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(this.conn)
	_, _ = fmt.Fprintln(b, log...)
	this.ctx.Print(l, "c_ctrl", b)
}

// if need async handle message must copy msg first
func (this *c_ctrl) handle_message(msg []byte) {
	if this.is_closed() {
		return
	}
	if len(msg) < 1 {
		this.log(logger.EXPLOIT, protocol.ERR_INVALID_MSG_SIZE)
		this.Close()
		return
	}
	switch msg[0] {
	case protocol.CTRL_REPLY:
		this.handle_reply(msg[1:])
	case protocol.CTRL_HEARTBEAT:
		this.handle_heartbeat()
	case protocol.CTRL_TRUST_NODE_DATA:

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
	}
}

func (this *c_ctrl) handle_heartbeat() {
	// <security> fake flow like client
	fake_size := 64 + this.random.Int(256)
	// size(4 Bytes) + heartbeat(1 byte) + fake data
	this.buffer.Reset()
	this.buffer.Write(convert.Uint32_Bytes(uint32(1 + fake_size)))
	this.buffer.WriteByte(protocol.NODE_HEARTBEAT)
	this.buffer.Write(this.random.Bytes(fake_size))
	// send
	_ = this.conn.SetWriteDeadline(time.Now().Add(protocol.SEND_TIMEOUT))
	_, _ = this.conn.Write(this.buffer.Bytes())
}

func (this *c_ctrl) reply(id, reply []byte) {
	if this.is_closed() {
		return
	}
	// size(4 Bytes) + NODE_REPLY(1 byte) + msg_id(2 bytes)
	l := len(reply)
	b := make([]byte, 7+l)
	copy(b, convert.Uint32_Bytes(uint32(3+l))) // write size
	b[4] = protocol.NODE_REPLY
	copy(b[5:7], id)
	copy(b[7:], reply)
	_ = this.conn.SetWriteDeadline(time.Now().Add(protocol.SEND_TIMEOUT))
	_, _ = this.conn.Write(b)
}

// msg_id(2 bytes) + data
func (this *c_ctrl) handle_reply(reply []byte) {
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
		this.reply_timer.Stop()
	case <-this.reply_timer.C:
		this.log(logger.EXPLOIT, protocol.ERR_RECV_INVALID_REPLY)
		this.Close()
	}
}

// send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (this *c_ctrl) Send(cmd uint8, data []byte) ([]byte, error) {
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
				_ = this.conn.SetWriteDeadline(time.Now().Add(protocol.SEND_TIMEOUT))
				_, err := this.conn.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				this.slots[id].Timer.Reset(protocol.RECV_TIMEOUT)
				select {
				case r := <-this.slots[id].Reply:
					this.slots[id].Timer.Stop()
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

func (this *c_ctrl) handle_trust() {

}

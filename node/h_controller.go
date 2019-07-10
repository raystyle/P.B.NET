package node

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
)

func (this *server) serve_ctrl(conn *xnet.Conn) {
	c := &c_ctrl{
		ctx:         this.ctx,
		conn:        conn,
		random:      random.New(),
		stop_signal: make(chan struct{}),
	}
	this.add_ctrl(c)
	// TODO don't print
	this.log(logger.INFO, &s_log{c: conn, l: "controller connected"})
	defer func() {
		this.del_ctrl("", c)
		this.log(logger.INFO, &s_log{c: conn, l: "controller disconnected"})
	}()
	// init slot
	for i := 0; i < protocol.SLOT_SIZE; i++ {
		s := &protocol.Slot{
			Available: make(chan struct{}, 1),
			Reply:     make(chan []byte, 1),
		}
		s.Available <- struct{}{}
		c.slots[i] = s
	}
	protocol.Handle_Conn(conn, c.handle_message, c.Close)
}

// controller client
type c_ctrl struct {
	ctx         *NODE
	conn        *xnet.Conn
	slots       [protocol.SLOT_SIZE]*protocol.Slot
	random      *random.Generator
	buffer      bytes.Buffer
	in_close    int32
	close_once  sync.Once
	stop_signal chan struct{}
}

func (this *c_ctrl) Info() *xnet.Info {
	return this.conn.Info()
}

func (this *c_ctrl) Close() {
	this.close_once.Do(func() {
		atomic.StoreInt32(&this.in_close, 1)
		close(this.stop_signal)
		_ = this.conn.Close()
	})
}

func (this *c_ctrl) is_closed() bool {
	return atomic.LoadInt32(&this.in_close) != 0
}

func (this *c_ctrl) logf(l logger.Level, format string, log ...interface{}) {
	this.ctx.Printf(l, "c_ctrl", format, log...)
}

func (this *c_ctrl) log(l logger.Level, log ...interface{}) {
	this.ctx.Print(l, "c_ctrl", log...)
}

func (this *c_ctrl) logln(l logger.Level, log ...interface{}) {
	this.ctx.Println(l, "c_ctrl", log...)
}

// if need async handle message must copy msg first
func (this *c_ctrl) handle_message(msg []byte) {
	if this.is_closed() {
		return
	}
	if len(msg) < 1 {
		l := &s_log{c: this.conn, l: "invalid message size"}
		this.log(logger.EXPLOIT, l)
		this.Close()
		return
	}
	switch msg[0] {
	case protocol.CTRL_REPLY:
		this.handle_reply(msg[1:])
	case protocol.CTRL_HEARTBEAT:
		this.handle_heartbeat()
	case protocol.ERR_NULL_MSG:
		l := &s_log{c: this.conn, l: "receive null message"} // TODO protocol err
		this.log(logger.EXPLOIT, l)
		this.Close()
	case protocol.ERR_TOO_BIG_MSG:
		l := &s_log{c: this.conn, l: "receive too big message"}
		this.log(logger.EXPLOIT, l)
		this.Close()
	default:
		l := &s_log{c: this.conn, l: "receive unknown command"}
		this.log(logger.EXPLOIT, l, msg[1:])
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
	_, _ = this.conn.Write(this.buffer.Bytes())
}

func (this *c_ctrl) reply(id, reply []byte) {
	if this.is_closed() {
		return
	}
	// size(4 Bytes) + NODE_REPLY(1 byte) + msg_id(2 bytes)
	l := len(reply)
	b := make([]byte, 7+l)
	copy(b, convert.Uint16_Bytes(uint16(3+l))) // write size
	b[4] = protocol.NODE_REPLY
	copy(b[5:7], id)
	copy(b[7:], reply)
	_, _ = this.conn.Write(b)
}

// msg_id(2 bytes) + data
func (this *c_ctrl) handle_reply(reply []byte) {
	const (
		max_id = protocol.SLOT_SIZE - 1
	)
	l := len(reply)
	if l < 2 {
		l := &s_log{c: this.conn, l: "receive invalid message id size"}
		this.log(logger.EXPLOIT, l)
		this.Close()
		return
	}
	id := int(convert.Bytes_Uint16(reply[:2]))
	if id > max_id {
		l := &s_log{c: this.conn, l: "receive invalid message id"}
		this.log(logger.EXPLOIT, l)
		this.Close()
		return
	}
	// must copy
	r := make([]byte, l-2)
	copy(r, reply[2:])
	this.slots[id].Reply <- r
}

// send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (this *c_ctrl) Send(cmd uint8, data []byte) ([]byte, error) {
	if this.is_closed() {
		return nil, errors.New("connection closed")
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
				_, err := this.conn.Write(b)
				if err != nil {
					return nil, err
				}
				// wait for reply
				select {
				case r := <-this.slots[id].Reply:
					this.slots[id].Available <- struct{}{}
					return r, nil
				case <-time.After(time.Minute):
					this.Close()
					return nil, errors.New("receive reply timeout")
				case <-this.stop_signal:
					return nil, errors.New("connection closed")
				}
			case <-this.stop_signal:
				return nil, errors.New("connection closed")
			default:
			}
		}
		// if full wait 1 second
		select {
		case <-time.After(time.Second):
		case <-this.stop_signal:
			return nil, errors.New("connection closed")
		}
	}
}

package node

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

const (
	// message id is uint16 < 65536
	slot_size = 256
)

func (this *server) serve_ctrl(conn *xnet.Conn) {
	c := &c_ctrl{
		ctx:         this.ctx,
		conn:        conn,
		stop_signal: make(chan struct{}, 1),
	}
	this.add_ctrl(c)
	// TODO don't print
	this.log(logger.INFO, &s_log{c: conn, l: "controller connected"})
	defer func() {
		this.del_ctrl("", c)
		this.log(logger.INFO, &s_log{c: conn, l: "controller disconnected"})
	}()
	// init slot
	for i := 0; i < slot_size; i++ {
		s := &slot{
			available: make(chan struct{}, 1),
			reply:     make(chan []byte, 1),
		}
		s.available <- struct{}{}
		c.slots[i] = s
	}
	protocol.Handle_Message(conn, c.handle_message)
}

// controller client
type c_ctrl struct {
	ctx         *NODE
	conn        *xnet.Conn
	slots       [slot_size]*slot
	in_close    int32
	close_once  sync.Once
	stop_signal chan struct{}
}

type slot struct {
	available chan struct{}
	reply     chan []byte
}

func (this *c_ctrl) Info() *xnet.Info {
	return this.conn.Info()
}

func (this *c_ctrl) Close() {
	atomic.StoreInt32(&this.in_close, 1)
	// more
	this.close()
}

func (this *c_ctrl) Kill() {
	atomic.StoreInt32(&this.in_close, 1)
	this.close()
}

func (this *c_ctrl) close() {
	this.close_once.Do(func() {
		_ = this.conn.Close()
		close(this.stop_signal)
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
	if len(msg) < 1 {
		l := &s_log{c: this.conn, l: "invalid message size"}
		this.log(logger.EXPLOIT, l)
		this.Kill()
		return
	}
	switch msg[0] {
	case protocol.CTRL_REPLY:
		this.handle_reply(msg[1:])
	case protocol.CTRL_HEARTBEAT:
		this.handle_heartbeat()
	case protocol.ERR_NULL_MESSAGE:
		l := &s_log{c: this.conn, l: "receive null message"}
		this.log(logger.EXPLOIT, l)
		this.Kill()
	case protocol.ERR_TOO_BIG_MESSAGE:
		l := &s_log{c: this.conn, l: "receive too big message"}
		this.log(logger.EXPLOIT, l)
		this.Kill()
	default:
		l := &s_log{c: this.conn, l: "receive unknown command"}
		this.log(logger.EXPLOIT, l)
		this.Kill()
	}
}

var node_heartbeat = []byte{0, 0, 0, 1, protocol.NODE_HEARTBEAT}

func (this *c_ctrl) handle_heartbeat() {
	_, _ = this.conn.Write(node_heartbeat)
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
		max_id = slot_size - 1
	)
	l := len(reply)
	if l < 2 {
		l := &s_log{c: this.conn, l: "receive invalid message id size"}
		this.log(logger.EXPLOIT, l)
		this.Kill()
		return
	}
	id := int(convert.Bytes_Uint16(reply[:2]))
	if id > max_id {
		l := &s_log{c: this.conn, l: "receive invalid message id"}
		this.log(logger.EXPLOIT, l)
		this.Kill()
		return
	}
	// must copy
	r := make([]byte, l-2)
	copy(r, reply[2:])
	this.slots[id].reply <- r
}

// send command and receive reply
// size(4 Bytes) + command(1 Byte) + msg_id(2 bytes) + data
func (this *c_ctrl) Send(cmd uint8, data []byte) ([]byte, error) {
	if this.is_closed() {
		return nil, errors.New("connection closed")
	}
	for {
		for id := 0; id < slot_size; id++ {
			select {
			case <-this.slots[id].available:
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
				case r := <-this.slots[id].reply:
					this.slots[id].available <- struct{}{}
					return r, nil
				case <-time.After(time.Minute):
					this.Kill()
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

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
	send_queue_size = 256
)

func (this *server) serve_ctrl(conn *xnet.Conn) {
	c := &c_ctrl{ctx: this.ctx, conn: conn}
	this.add_ctrl(c)
	// TODO don't print
	this.log(logger.INFO, &s_log{c: conn, l: "controller connected"})
	defer func() {
		this.del_ctrl("", c)
		this.log(logger.INFO, &s_log{c: conn, l: "controller disconnected"})
	}()
	c.send_queue = make(chan []byte, send_queue_size)
	c.slots = make(map[uint32]*slot) // fast access
	// init slot
	for i := 0; i < send_queue_size; i++ {
		s := &slot{
			available: make(chan struct{}, 1),
			reply:     make(chan []byte, 1),
		}
		s.available <- struct{}{}
		c.slots[uint32(i)] = s
	}
	c.stop_signal = make(chan struct{}, 1)
	protocol.Handle_Message(conn, c.handle_message)
}

// controller client
type c_ctrl struct {
	ctx         *NODE
	conn        *xnet.Conn
	send_queue  chan []byte
	slots       map[uint32]*slot
	in_close    int32
	stop_signal chan struct{}
	wg          sync.WaitGroup
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
	close(this.send_queue)
	_ = this.conn.Close()
	this.wg.Wait()
}

func (this *c_ctrl) Kill() {
	atomic.StoreInt32(&this.in_close, 1)
	close(this.send_queue)
	_ = this.conn.Close()
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
	case protocol.ERR_NULL_MESSAGE:
		l := &s_log{c: this.conn, l: "receive null message"}
		this.log(logger.EXPLOIT, l)
	case protocol.ERR_TOO_BIG_MESSAGE:
		l := &s_log{c: this.conn, l: "receive too big message"}
		this.log(logger.EXPLOIT, l)
	default:
		l := &s_log{c: this.conn, l: "receive unknown command"}
		this.log(logger.EXPLOIT, l)
		this.Kill()
	}
}

func (this *c_ctrl) reply(id, reply []byte) {
	// size(4 Bytes) + NODE_REPLY(1 byte) + id(4 bytes)
	l := len(reply)
	b := make([]byte, 9+l)
	copy(b, convert.Uint32_Bytes(uint32(5+l))) // write size
	b[4] = protocol.NODE_REPLY
	copy(b[5:9], id)
	copy(b[9:], reply)
	this.send_queue <- b
}

func (this *c_ctrl) handle_reply(reply []byte) {
	if len(reply) < 4 {
		l := &s_log{c: this.conn, l: "receive invalid message id size"}
		this.log(logger.EXPLOIT, l)
		return
	}
	this.slots[convert.Bytes_Uint32(reply[:4])].reply <- reply[4:]
}

// send command and receive reply
func (this *c_ctrl) Send(cmd uint8, data []byte, timeout int) ([]byte, error) {
	if this.is_closed() {
		return nil, errors.New("connection closed")
	}
	for id, s := range this.slots {
		select {
		case <-s.available:
			// size(4) + command(1) + msg_id(4) + data
			l := len(data)
			b := make([]byte, 9+l)
			copy(b, convert.Uint32_Bytes(uint32(5+l))) // write size
			b[4] = cmd
			copy(b[5:9], convert.Uint32_Bytes(id))
			copy(b[:], data)
			this.send_queue <- b
			select {
			case r := <-s.reply:
				s.available <- struct{}{}
				return r, nil
			case <-time.After(time.Duration(timeout) * time.Second):
				// TODO more think
				s.available <- struct{}{}
				return nil, errors.New("receive reply timeout")
			case <-this.stop_signal:
				return nil, errors.New("connection closed")
			}
		default:
		}
	}
	// WARNING
	return nil, errors.New("slot is full")
}

func (this *c_ctrl) sender() {
	defer this.wg.Done()
	var (
		msg []byte
		err error
	)
	for msg = range this.send_queue {
		_, err = this.conn.Write(msg)
		if err != nil {
			return
		}
	}
}

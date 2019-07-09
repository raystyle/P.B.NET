package node

import (
	"sync"
	"sync/atomic"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

const (
	send_queue_size = 4096
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
	c.replys = make(map[uint64]chan<- []byte)
	c.stop_signal = make(chan struct{}, 1)
	protocol.Handle_Message(conn, c.handle_message)
}

// controller client
type c_ctrl struct {
	ctx         *NODE
	conn        *xnet.Conn
	msg_id      uint64
	send_queue  chan []byte
	replys      map[uint64]chan<- []byte
	send_m      sync.Mutex
	in_close    int32
	stop_signal chan struct{}
	wg          sync.WaitGroup
}

func (this *c_ctrl) Info() *xnet.Info {
	return this.conn.Info()
}

func (this *c_ctrl) Close() {
	atomic.StoreInt32(&this.in_close, 1)
	_ = this.conn.Close()
	this.wg.Wait()
}

func (this *c_ctrl) Kill() {
	atomic.StoreInt32(&this.in_close, 1)
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

func (this *c_ctrl) reply(id, reply []byte) error {
	// command(1 byte) + id(8 bytes)
	l := len(reply)
	b := make([]byte, 9+l)
	// write size
	copy(b, convert.Uint32_Bytes(uint32(9+l)))
	b[4] = protocol.NODE_REPLY
	copy(b[5:13], id)
	copy(b[13:], reply)
	_, err := this.conn.Write(b)
	return err
}

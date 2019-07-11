package node

import (
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
	ctx  *NODE
	conn *xnet.Conn
	// send_queue  chan []byte
	slots       []*protocol.Slot
	reply_timer *time.Timer
	random      *random.Generator
	in_close    int32
	close_once  sync.Once
	stop_signal chan struct{}
	wg          sync.WaitGroup
}

func (this *server) serve_ctrl(conn *xnet.Conn) {
	c := &c_ctrl{
		ctx:  this.ctx,
		conn: conn,
		// send_queue:  make(chan []byte, 4*protocol.SLOT_SIZE),
		slots:       make([]*protocol.Slot, protocol.SLOT_SIZE),
		reply_timer: time.NewTimer(time.Second),
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
	// init & slot
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
	// c.wg.Add(1)
	// go c.sender()
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
	case protocol.TEST_MSG:
		if len(msg) < 3 {
			l := &s_log{c: this.conn, l: "receive invalid test message"}
			this.log(logger.EXPLOIT, l)
		}
		this.reply(msg[1:3], msg[3:])
	default:
		l := &s_log{c: this.conn, l: "receive unknown command"}
		this.log(logger.EXPLOIT, l, msg[1:])
		this.Close()
	}
}

/*
func (this *c_ctrl) sender() {
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
*/

func (this *c_ctrl) handle_heartbeat() {
	// <security> fake flow like client
	fake_size := 64 + this.random.Int(256)
	// size(4 Bytes) + heartbeat(1 byte) + fake data
	b := make([]byte, 5+fake_size)
	copy(b, convert.Uint32_Bytes(uint32(1+fake_size)))
	b[4] = protocol.NODE_HEARTBEAT
	copy(b[5:], this.random.Bytes(fake_size))
	// send
	// select {
	// case this.send_queue <- b:
	// case <-time.After(protocol.SEND_TIMEOUT):
	// this.log(logger.WARNING, protocol.ERR_SEND_TIMEOUT)
	// _ = this.conn.Close()
	// }
	_, _ = this.conn.Write(b)
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
	_, _ = this.conn.Write(b)
	/*
		select {
		case this.send_queue <- b:
		case <-time.After(protocol.SEND_TIMEOUT):
			this.log(logger.WARNING, protocol.ERR_SEND_TIMEOUT)
			this.Close()
		}
	*/
}

// msg_id(2 bytes) + data
func (this *c_ctrl) handle_reply(reply []byte) {
	l := len(reply)
	if l < 2 {
		l := &s_log{c: this.conn, l: "receive invalid message id size"}
		this.log(logger.EXPLOIT, l)
		this.Close()
		return
	}
	id := int(convert.Bytes_Uint16(reply[:2]))
	if id > protocol.MAX_MSG_ID {
		l := &s_log{c: this.conn, l: "receive invalid message id"}
		this.log(logger.EXPLOIT, l)
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
				_, err := this.conn.Write(b)
				if err != nil {
					return nil, err
				}
				/*
					select {
					case this.send_queue <- b:
					case <-time.After(protocol.SEND_TIMEOUT):
						this.log(logger.WARNING, protocol.ERR_SEND_TIMEOUT)
						this.Close()
						return nil, protocol.ERR_SEND_TIMEOUT
					}
				*/
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

package node

import (
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

func (this *server) serve_ctrl(conn *xnet.Conn) {
	c := &c_ctrl{ctx: this.ctx, conn: conn}
	this.add_ctrl(c)
	// TODO don't print
	this.logln(logger.INFO, &s_log{c: conn, l: "controller connected"})
	defer func() {
		this.del_ctrl("", c)
		this.logln(logger.INFO, &s_log{c: conn, l: "controller disconnected"})
	}()
	protocol.Handle_Message(conn, c.handle_message)
}

// controller client
type c_ctrl struct {
	ctx  *NODE
	conn *xnet.Conn
}

func (this *c_ctrl) Info() *xnet.Info {
	return this.conn.Info()
}

func (this *c_ctrl) Close() {
	_ = this.conn.Close()
}

func (this *c_ctrl) Kill() {
	_ = this.conn.Close()
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
		this.logln(logger.EXPLOIT, l)
		this.Kill()
		return
	}
	switch msg[0] {

	case protocol.ERR_NULL_MESSAGE:
		l := &s_log{c: this.conn, l: "receive null message"}
		this.logln(logger.EXPLOIT, l)
	case protocol.ERR_TOO_BIG_MESSAGE:
		l := &s_log{c: this.conn, l: "receive too big message"}
		this.logln(logger.EXPLOIT, l)
	default:
		l := &s_log{c: this.conn, l: "receive unknown command"}
		this.logln(logger.EXPLOIT, l)
		this.Kill()
	}
}

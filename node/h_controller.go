package node

import (
	"time"

	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

func (this *server) handle_ctrl(conn *xnet.Conn) {
	_ = conn.SetDeadline(time.Time{})
	c := &v_ctrl{conn: conn}
	this.add_ctrl(c)
	// TODO don't print
	this.logln(logger.INFO, &hs_log{c: conn, l: "controller connected"})
	defer func() {
		this.del_ctrl("", c)
		this.logln(logger.INFO, &hs_log{c: conn, l: "controller disconnected"})
	}()
	c.serve()
}

type v_ctrl struct {
	conn *xnet.Conn
}

func (this *v_ctrl) Info() *xnet.Info {
	return this.conn.Info()
}

func (this *v_ctrl) Close() {

}

func (this *v_ctrl) Kill() {
	_ = this.conn.Close()
}

func (this *v_ctrl) serve() {
	protocol.Handle_Message(this.conn, this.handle_message)
}

func (this *v_ctrl) handle_message(msg []byte) {

}

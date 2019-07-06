package node

import (
	"time"

	"project/internal/protocol"
	"project/internal/xnet"
)

func (this *server) handle_ctrl(conn *xnet.Conn) {
	_ = conn.SetDeadline(time.Time{})
	ver := conn.Info().Version
	switch {
	case ver == protocol.V1_0_0:
		c := &v1_ctrl{conn: conn}
		this.add_ctrl(c)
		defer func() {
			this.del_ctrl("", c)

		}()
		c.serve()
	}
}

type v1_ctrl struct {
	conn *xnet.Conn
}

func (this *v1_ctrl) Info() *xnet.Info {
	return this.conn.Info()
}

func (this *v1_ctrl) Close() {

}

func (this *v1_ctrl) Kill() {
	_ = this.conn.Close()
}

func (this *v1_ctrl) serve() {
	v1_handle_message(this.conn, this.handle_message)
}

func (this *v1_ctrl) handle_message(msg []byte) {

}

package controller

import (
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

type client struct {
	ctx  *CTRL
	node *bootstrap.Node
	guid []byte
	conn *xnet.Conn
	ver  protocol.Version
}

// Node_GUID != nil for sync or other
// Node_GUID = nil for trust node
// Node_GUID = controller guid for discovery
type client_config struct {
	Node      *bootstrap.Node
	Node_GUID []byte
	xnet.Config
}

func new_client(ctx *CTRL, c *client_config) (*client, error) {
	c.Network = c.Node.Network
	c.Address = c.Node.Address
	conn, err := xnet.Dial(c.Node.Mode, &c.Config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	client := &client{
		ctx:  ctx,
		node: c.Node,
		guid: c.Node_GUID,
	}
	err_chan := make(chan error, 1)
	go func() {
		xconn, err := client.handshake(conn)
		if err != nil {
			err_chan <- err
			return
		}
		client.conn = xconn
		close(err_chan)
	}()
	select {
	case err = <-err_chan:
		if err != nil {
			return nil, err
		}
	case <-time.After(time.Minute):
		return nil, errors.New("handshake timeout")
	}
	return client, nil
}

func (this *client) logf(l logger.Level, format string, log ...interface{}) {
	this.ctx.Printf(l, "client", format, log...)
}

func (this *client) log(l logger.Level, log ...interface{}) {
	this.ctx.Print(l, "client", log...)
}

func (this *client) logln(l logger.Level, log ...interface{}) {
	this.ctx.Println(l, "client", log...)
}

func (this *client) Close() {
	_ = this.conn.Close()
}

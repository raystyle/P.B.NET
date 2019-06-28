package controller

import (
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

type client struct {
	ctx  *CTRL
	node *bootstrap.Node
	conn *xnet.Conn
}

func new_client(ctx *CTRL, n *bootstrap.Node) *client {
	return &client{
		ctx:  ctx,
		node: n,
	}
}

func (this *client) Connect(skip_verify bool) error {
	c := &xnet.Config{
		Network: this.node.Network,
		Address: this.node.Address,
	}
	conn, err := xnet.Dial(this.node.Mode, c)
	if err != nil {
		return err
	}
	xconn, err := this.handshake(conn, skip_verify)
	if err != nil {
		return errors.WithMessage(err, "handshake failed")
	}
	this.conn = xconn
	return nil
}

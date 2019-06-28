package controller

import (
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/xnet"
)

type client struct {
	ctx  *CTRL
	node *bootstrap.Node
	guid []byte
	conn *xnet.Conn
}

// guid = nil for discovery
func new_client(ctx *CTRL, n *bootstrap.Node, guid []byte) *client {
	return &client{
		ctx:  ctx,
		node: n,
		guid: guid,
	}
}

// skip_verify for trust genesis node and then sign it
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

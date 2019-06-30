package controller

import (
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
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

// guid != nil for sync or other
// guid = nil for trust node
// guid = controller guid for discovery
func new_client(ctx *CTRL, n *bootstrap.Node, guid []byte) (*client, error) {
	config := &xnet.Config{
		Network: n.Network,
		Address: n.Address,
	}
	conn, err := xnet.Dial(n.Mode, config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	client := &client{
		ctx:  ctx,
		node: n,
		guid: guid,
	}
	err_chan := make(chan error, 1)
	go func() {
		xconn, err := client.handshake(conn)
		if err != nil {
			err_chan <- err
		}
		client.conn = xconn
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

func (this *client) Info() {

}

func (this *client) Close() {
	_ = this.conn.Close()
}

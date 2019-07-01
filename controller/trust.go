package controller

import (
	"fmt"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/protocol"
)

// Trust_Node is used to trust Genesis Node
func (this *CTRL) Trust_Node(n *bootstrap.Node) error {
	c := &client_config{Node: n}
	c.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(this, c)
	if err != nil {
		return errors.WithMessage(err, "connect node failed")
	}
	return client.Trust_Node(n)
}

func (this *client) Trust_Node(n *bootstrap.Node) error {
	defer this.Close()
	var err error
	switch {
	case this.ver == protocol.V1_0_0:
		err = this.v1_trust_node(n)
	}
	return err
}

func (this *client) v1_trust_node(n *bootstrap.Node) error {
	fmt.Println("trust ok")
	return nil
}

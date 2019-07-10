package controller

import (
	"fmt"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
)

// Trust_Node is used to trust Genesis Node
func (this *CTRL) Trust_Node(n *bootstrap.Node) error {
	c := &client_cfg{Node: n}
	c.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(this, c)
	if err != nil {
		return errors.WithMessage(err, "connect node failed")
	}
	defer client.Close()
	return client.Trust_Node(n)
}

func (this *client) Trust_Node(n *bootstrap.Node) error {
	var err error
	fmt.Println("trust ok")
	return err
}

package controller

import (
	"github.com/pkg/errors"

	"project/internal/bootstrap"
)

// Trust_Node is used to trust Genesis Node
func (this *CTRL) Trust_Node(n *bootstrap.Node) error {
	client := new_client(this, n, nil)
	err := client.Connect(true)
	if err != nil {
		return errors.WithMessage(err, "connect node failed")
	}
	return nil
}

package controller

import (
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
)

// Trust_Node is used to trust Genesis Node
func (this *CTRL) Trust_Node(n *bootstrap.Node) error {
	c := &client_cfg{Node: n}
	c.TLS_Config.InsecureSkipVerify = true
	client, err := new_client(this, c)
	if err != nil {
		return errors.Wrap(err, "connect node failed")
	}
	defer client.Close()
	// send trust node command
	reply, err := client.Send(protocol.CTRL_TRUST_NODE, nil)
	if err != nil {
		return errors.Wrap(err, "send trust node command failed")
	}
	nor := &messages.Node_Online_Request{}
	err = msgpack.Unmarshal(reply, nor)
	if err != nil {
		err = errors.Wrap(err, "invalid node online request")
		this.Print(logger.EXPLOIT, "trust_node", err)
		return err
	}
	err = nor.Validate()
	if err != nil {
		err = errors.Wrap(err, "validate node online request failed")
		this.Print(logger.EXPLOIT, "trust_node", err)
		return err
	}

	// calculate aes key
	// this.global.Key_Exchange()
	return nil
}

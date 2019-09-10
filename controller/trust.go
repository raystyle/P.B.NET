package controller

import (
	"bytes"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
)

// TrustNode is used to trust Genesis Node
func (ctrl *CTRL) TrustNode(node *bootstrap.Node) (*messages.NodeOnlineRequest, error) {
	cfg := &clientCfg{Node: node}
	cfg.TLSConfig.InsecureSkipVerify = true
	client, err := newClient(ctrl, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "connect node failed")
	}
	defer client.Close()
	// send trust node command
	reply, err := client.Send(protocol.CtrlTrustNode, nil)
	if err != nil {
		return nil, errors.Wrap(err, "send trust node command failed")
	}
	req := messages.NodeOnlineRequest{}
	err = msgpack.Unmarshal(reply, &req)
	if err != nil {
		err = errors.Wrap(err, "invalid node online request")
		ctrl.Print(logger.EXPLOIT, "trust_node", err)
		return nil, err
	}
	err = req.Validate()
	if err != nil {
		err = errors.Wrap(err, "validate node online request failed")
		ctrl.Print(logger.EXPLOIT, "trust_node", err)
		return nil, err
	}
	return &req, nil
}

func (ctrl *CTRL) ConfirmTrustNode(node *bootstrap.Node, req *messages.NodeOnlineRequest) error {
	cfg := &clientCfg{Node: node}
	cfg.TLSConfig.InsecureSkipVerify = true
	client, err := newClient(ctrl, cfg)
	if err != nil {
		return errors.Wrap(err, "connect node failed")
	}
	defer client.Close()
	// issue certificates
	cert := ctrl.issueCertificate(node, req.GUID)
	// send response
	reply, err := client.Send(protocol.CtrlTrustNodeData, cert)
	if err != nil {
		return errors.Wrap(err, "send trust node data failed")
	}
	if !bytes.Equal(reply, messages.OnlineSucceed) {
		return errors.New("trust node failed")
	}
	// calculate aes key
	aesKey, err := ctrl.global.KeyExchange(req.KexPublicKey)
	if err != nil {
		err = errors.Wrap(err, "calculate session key failed")
		ctrl.Print(logger.EXPLOIT, "trust_node", err)
		return err
	}
	// TODO broadcast

	// insert node
	return ctrl.InsertNode(&mNode{
		GUID:       req.GUID,
		PublicKey:  req.PublicKey,
		SessionKey: aesKey,
	})
}

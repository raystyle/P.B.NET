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
// receive host info for confirm
func (ctrl *CTRL) TrustNode(node *bootstrap.Node) (*messages.NodeOnlineRequest, error) {
	cfg := &clientCfg{Node: node}
	client, err := newClient(ctrl, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "connect node failed")
	}
	defer client.Close()
	// send trust node command
	reply, err := client.Send(protocol.CtrlTrustNode, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "send trust node command failed")
	}
	req := messages.NodeOnlineRequest{}
	err = msgpack.Unmarshal(reply, &req)
	if err != nil {
		err = errors.Wrap(err, "invalid node online request")
		ctrl.Print(logger.Exploit, "trust node", err)
		return nil, err
	}
	err = req.Validate()
	if err != nil {
		err = errors.Wrap(err, "validate node online request failed")
		ctrl.Print(logger.Exploit, "trust node", err)
		return nil, err
	}
	return &req, nil
}

// ConfirmTrustNode is used to confirm trust node
// issue certificates and insert to database
func (ctrl *CTRL) ConfirmTrustNode(node *bootstrap.Node, req *messages.NodeOnlineRequest) error {
	cfg := &clientCfg{Node: node}
	cfg.TLSConfig.InsecureSkipVerify = true
	client, err := newClient(ctrl, cfg)
	if err != nil {
		return errors.WithMessage(err, "connect node failed")
	}
	defer client.Close()
	// issue certificates
	cert := ctrl.issueCertificate(node.Address, req.GUID)
	// send response
	reply, err := client.Send(protocol.CtrlTrustNodeData, cert)
	if err != nil {
		return errors.WithMessage(err, "send trust node data failed")
	}
	if !bytes.Equal(reply, messages.OnlineSucceed) {
		return errors.Errorf("trust node failed: %s", string(reply))
	}
	// calculate aes key
	sKey, err := ctrl.global.KeyExchange(req.KexPublicKey)
	if err != nil {
		err = errors.Wrap(err, "calculate session key failed")
		ctrl.Print(logger.Exploit, "trust node", err)
		return err
	}
	// insert node
	return ctrl.db.InsertNode(&mNode{
		GUID:        req.GUID,
		PublicKey:   req.PublicKey,
		SessionKey:  sKey,
		IsBootstrap: true,
	})
}

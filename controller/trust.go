package controller

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/security"
)

// TrustNode is used to trust Node, receive system info for confirm it.
// usually for the initial node or the test
func (ctrl *CTRL) TrustNode(
	ctx context.Context,
	node *bootstrap.Node,
) (*messages.NodeRegisterRequest, error) {
	client, err := ctrl.newClient(ctx, node, nil, nil)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	// send trust node command
	reply, err := client.Send(protocol.CtrlTrustNode, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to send trust node command")
	}
	req := messages.NodeRegisterRequest{}
	err = msgpack.Unmarshal(reply, &req)
	if err != nil {
		err = errors.Wrap(err, "invalid node register request msgpack data")
		ctrl.logger.Print(logger.Exploit, "trust node", err)
		return nil, err
	}
	err = req.Validate()
	if err != nil {
		err = errors.Wrap(err, "invalid node register request")
		ctrl.logger.Print(logger.Exploit, "trust node", err)
		return nil, err
	}
	return &req, nil
}

// ConfirmTrustNode is used to confirm trust node,
// issue certificates and insert to database
func (ctrl *CTRL) ConfirmTrustNode(
	ctx context.Context,
	node *bootstrap.Node,
	req *messages.NodeRegisterRequest,
) error {
	client, err := ctrl.newClient(ctx, node, nil, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	// issue certificates
	cert := protocol.Certificate{
		GUID:      req.GUID,
		PublicKey: req.PublicKey,
	}
	privateKey := ctrl.global.PrivateKey()
	defer security.CoverBytes(privateKey)
	err = protocol.IssueCertificate(&cert, privateKey)
	if err != nil {
		return err
	}
	security.CoverBytes(privateKey)
	// send certificate
	reply, err := client.Send(protocol.CtrlSetNodeCert, cert.Encode())
	if err != nil {
		return errors.WithMessage(err, "failed to set node certificate")
	}
	if bytes.Compare(reply, []byte{messages.RegisterResultAccept}) != 0 {
		return errors.Errorf("failed to trust node: %s", reply)
	}
	// calculate session key
	sessionKey, err := ctrl.global.KeyExchange(req.KexPublicKey)
	if err != nil {
		err = errors.Wrap(err, "failed to calculate session key")
		ctrl.logger.Print(logger.Exploit, "trust node", err)
		return err
	}
	return ctrl.database.InsertNode(&mNode{
		GUID:        req.GUID,
		PublicKey:   req.PublicKey,
		SessionKey:  sessionKey,
		IsBootstrap: true,
	})
}

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
)

// TrustNode is used to trust Genesis Node
// receive host info for confirm
func (ctrl *CTRL) TrustNode(
	ctx context.Context,
	node *bootstrap.Node,
) (*messages.NodeRegisterRequest, error) {
	client, err := newClient(ctx, ctrl, node, nil, nil)
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

// ConfirmTrustNode is used to confirm trust node
// issue certificates and insert to database
func (ctrl *CTRL) ConfirmTrustNode(
	ctx context.Context,
	node *bootstrap.Node,
	req *messages.NodeRegisterRequest,
) error {
	client, err := newClient(ctx, ctrl, node, nil, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	// issue certificates
	cert := ctrl.issueCertificate(node.Address, req.GUID)
	// send response
	reply, err := client.Send(protocol.CtrlSetNodeCert, cert)
	if err != nil {
		return errors.WithMessage(err, "failed to set node certificate")
	}
	if !bytes.Equal(reply, []byte{messages.RegisterResultAccept}) {
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

func (ctrl *CTRL) issueCertificate(address string, guid []byte) []byte {
	// sign certificate with node guid
	buffer := bytes.Buffer{}
	buffer.WriteString(address)
	buffer.Write(guid)
	certWithNodeGUID := ctrl.global.Sign(buffer.Bytes())
	// sign certificate with controller guid
	buffer.Truncate(len(address))
	buffer.Write(protocol.CtrlGUID)
	certWithCtrlGUID := ctrl.global.Sign(buffer.Bytes())
	// pack certificates
	// ed25519 signature with node guid
	// ed25519 signature with ctrl guid
	buffer.Reset()
	buffer.Write(certWithNodeGUID)
	buffer.Write(certWithCtrlGUID)
	return buffer.Bytes()
}

package controller

import (
	"bytes"
	"context"
	"io"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/convert"
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
		err = errors.Wrap(err, "invalid node online request")
		ctrl.logger.Print(logger.Exploit, "trust node", err)
		return nil, err
	}
	err = req.Validate()
	if err != nil {
		err = errors.Wrap(err, "validate node online request failed")
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
		return errors.WithMessage(err, "failed to send trust node certificate")
	}
	if !bytes.Equal(reply, messages.RegisterSucceed) {
		return errors.Errorf("failed to trust node: %s", string(reply))
	}
	// calculate aes key
	sKey, err := ctrl.global.KeyExchange(req.KexPublicKey)
	if err != nil {
		err = errors.Wrap(err, "failed to calculate session key")
		ctrl.logger.Print(logger.Exploit, "trust node", err)
		return err
	}
	return ctrl.db.InsertNode(&mNode{
		GUID:        req.GUID,
		PublicKey:   req.PublicKey,
		SessionKey:  sKey,
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
	// [2 byte uint16] size + signature with node guid
	// [2 byte uint16] size + signature with ctrl guid
	buffer.Reset()
	buffer.Write(convert.Uint16ToBytes(uint16(len(certWithNodeGUID))))
	buffer.Write(certWithNodeGUID)
	buffer.Write(convert.Uint16ToBytes(uint16(len(certWithCtrlGUID))))
	buffer.Write(certWithCtrlGUID)
	return buffer.Bytes()
}

func (ctrl *CTRL) verifyCertificate(cert []byte, address string, guid []byte) bool {
	// if guid = nil, skip verify
	if guid == nil {
		return true
	}
	reader := bytes.NewReader(cert)
	// read certificate size
	certSize := make([]byte, 2)
	_, err := io.ReadFull(reader, certSize)
	if err != nil {
		return false
	}
	// read certificate with node guid
	certWithNodeGUID := make([]byte, convert.BytesToUint16(certSize))
	_, err = io.ReadFull(reader, certWithNodeGUID)
	if err != nil {
		return false
	}
	// verify certificate
	buffer := bytes.Buffer{}
	buffer.WriteString(address)
	buffer.Write(guid)
	// switch certificate
	if bytes.Equal(guid, protocol.CtrlGUID) {
		// read cert size
		_, err = io.ReadFull(reader, certSize)
		if err != nil {
			return false
		}
		// read cert
		certWithCtrlGUID := make([]byte, convert.BytesToUint16(certSize))
		_, err = io.ReadFull(reader, certWithCtrlGUID)
		if err != nil {
			return false
		}
		if !ctrl.global.Verify(buffer.Bytes(), certWithCtrlGUID) {
			return false
		}
	} else {
		if !ctrl.global.Verify(buffer.Bytes(), certWithNodeGUID) {
			return false
		}
	}
	return true
}

package controller

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/security"
)

// TrustNode is used to trust Node, receive system info for confirm it.
// usually for the initial node or the test
// TODO add log
func (ctrl *Ctrl) TrustNode(
	ctx context.Context,
	listener *bootstrap.Listener,
) (*messages.NodeRegisterRequest, error) {
	client, err := ctrl.NewClient(ctx, listener, nil, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create client")
	}
	defer client.Close()
	// send trust node command
	reply, err := client.send(protocol.CtrlTrustNode, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to send trust node command")
	}
	if len(reply) < curve25519.ScalarSize+aes.BlockSize {
		// TODO add exploit
		return nil, errors.New("node send register request with invalid size")
	}
	// calculate role session key
	key, err := ctrl.global.KeyExchange(reply[:curve25519.ScalarSize])
	if err != nil {
		const format = "node send invalid register request\nerror: %s"
		return nil, errors.Errorf(format, err)
	}
	// decrypt role register request
	request, err := aes.CBCDecrypt(reply[curve25519.ScalarSize:], key, key[:aes.IVSize])
	if err != nil {
		const format = "node send invalid register request\nerror: %s"
		return nil, errors.Errorf(format, err)
	}
	nrr := messages.NodeRegisterRequest{}
	err = msgpack.Unmarshal(request, &nrr)
	if err != nil {
		// ctrl.logger.Print(logger.Exploit, "trust node", err)
		return nil, errors.Wrap(err, "invalid node register request")
	}
	err = nrr.Validate()
	if err != nil {
		// ctrl.logger.Print(logger.Exploit, "trust node", err)
		return nil, errors.Wrap(err, "invalid node register request")
	}
	return &nrr, nil
}

// ConfirmTrustNode is used to confirm trust node, register node
// TODO add log
func (ctrl *Ctrl) ConfirmTrustNode(
	ctx context.Context,
	listener *bootstrap.Listener,
	nrr *messages.NodeRegisterRequest,
) error {
	client, err := ctrl.NewClient(ctx, listener, nil, nil)
	if err != nil {
		return errors.WithMessage(err, "failed to create client")
	}
	defer client.Close()
	// register node
	certificate, err := ctrl.registerNode(nrr, true)
	if err != nil {
		return err
	}
	// send certificate
	reply, err := client.send(protocol.CtrlSetNodeCert, certificate.Encode())
	if err != nil {
		return errors.WithMessage(err, "failed to set node certificate")
	}
	if bytes.Compare(reply, []byte{messages.RegisterResultAccept}) != 0 {
		return errors.Errorf("failed to trust node: %s", reply)
	}
	return nil
}

func (ctrl *Ctrl) registerNode(
	nrr *messages.NodeRegisterRequest,
	bootstrap bool,
) (*protocol.Certificate, error) {
	failed := func(err error) error {
		return errors.Wrap(err, "failed to register node")
	}
	// issue certificate
	certificate := protocol.Certificate{
		GUID:      nrr.GUID,
		PublicKey: nrr.PublicKey,
	}
	privateKey := ctrl.global.PrivateKey()
	defer security.CoverBytes(privateKey)
	err := protocol.IssueCertificate(&certificate, privateKey)
	if err != nil {
		return nil, failed(err)
	}
	security.CoverBytes(privateKey)
	// calculate session key
	sessionKey, err := ctrl.global.KeyExchange(nrr.KexPublicKey)
	if err != nil {
		err = errors.WithMessage(err, "failed to calculate session key")
		ctrl.logger.Print(logger.Exploit, "register node", err)
		return nil, failed(err)
	}
	defer security.CoverBytes(sessionKey)
	err = ctrl.database.InsertNode(&mNode{
		GUID:         nrr.GUID[:],
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		SessionKey:   security.NewBytes(sessionKey),
		IsBootstrap:  bootstrap,
	})
	if err != nil {
		return nil, failed(err)
	}
	return &certificate, nil
}

// AcceptRegisterNode is used to accept register Node
// TODO add Log
func (ctrl *Ctrl) AcceptRegisterNode(nrr *messages.NodeRegisterRequest, bootstrap bool) error {
	certificate, err := ctrl.registerNode(nrr, bootstrap)
	if err != nil {
		return err
	}
	// broadcast Node register response
	resp := messages.NodeRegisterResponse{
		GUID:         nrr.GUID,
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		RequestTime:  nrr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultAccept,
		Certificate:  certificate.Encode(),
	}
	// TODO select node listeners
	err = ctrl.sender.Broadcast(messages.CMDBNodeRegisterResponse, resp)
	return errors.Wrap(err, "failed to accept register node")
}

// RefuseRegisterNode is used to refuse register Node, it will call firewall
func (ctrl *Ctrl) RefuseRegisterNode(nrr *messages.NodeRegisterRequest) error {
	resp := messages.NodeRegisterResponse{
		GUID:         nrr.GUID,
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		RequestTime:  nrr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultRefused,
		// padding for Validate()
		Certificate: make([]byte, protocol.CertificateSize),
	}
	err := ctrl.sender.Broadcast(messages.CMDBNodeRegisterResponse, resp)
	return errors.Wrap(err, "failed to refuse register node")
}

func (ctrl *Ctrl) registerBeacon(brr *messages.BeaconRegisterRequest) error {
	failed := func(err error) error {
		return errors.Wrap(err, "failed to register beacon")
	}
	// calculate session key
	sessionKey, err := ctrl.global.KeyExchange(brr.KexPublicKey)
	if err != nil {
		err = errors.WithMessage(err, "failed to calculate session key")
		ctrl.logger.Print(logger.Exploit, "register beacon", err)
		return failed(err)
	}
	defer security.CoverBytes(sessionKey)
	err = ctrl.database.InsertBeacon(&mBeacon{
		GUID:         brr.GUID[:],
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		SessionKey:   security.NewBytes(sessionKey),
	})
	if err != nil {
		return failed(err)
	}
	return nil
}

// AcceptRegisterBeacon is used to accept register Beacon.
// TODO add Log
func (ctrl *Ctrl) AcceptRegisterBeacon(brr *messages.BeaconRegisterRequest) error {
	err := ctrl.registerBeacon(brr)
	if err != nil {
		return err
	}
	// broadcast Beacon register response
	resp := messages.BeaconRegisterResponse{
		GUID:         brr.GUID,
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		RequestTime:  brr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultAccept,
	}
	// TODO select node listeners
	err = ctrl.sender.Broadcast(messages.CMDBBeaconRegisterResponse, resp)
	return errors.Wrap(err, "failed to accept register beacon")
}

// RefuseRegisterBeacon is used to refuse register Beacon, it will call firewall.
func (ctrl *Ctrl) RefuseRegisterBeacon(brr *messages.BeaconRegisterRequest) error {
	resp := messages.BeaconRegisterResponse{
		GUID:         brr.GUID,
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		RequestTime:  brr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultRefused,
	}
	err := ctrl.sender.Broadcast(messages.CMDBBeaconRegisterResponse, &resp)
	return errors.Wrap(err, "failed to refuse register beacon")
}
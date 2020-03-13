package controller

import (
	"bytes"
	"context"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/patch/msgpack"
	"project/internal/protocol"
	"project/internal/security"
)

// TrustNode is used to trust Node, receive system info for confirm it.
// usually for the Initial Node or the test.
func (ctrl *Ctrl) TrustNode(
	ctx context.Context,
	listener *bootstrap.Listener,
) (*NoticeNodeRegister, error) {
	// check exists
	const src = "trust-node"
	ctrl.logger.Printf(logger.Info, src, "listener: %s", listener)
	client, err := ctrl.NewClient(ctx, listener, nil, nil)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	// send trust node command
	reply, err := client.send(protocol.CtrlTrustNode, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to send trust node command")
	}
	// resolve node register request
	nrr, err := ctrl.resolveNodeRegisterRequest(reply)
	if err != nil {
		ctrl.logger.Printf(logger.Exploit, src, "%s\nlistener: %s", err, listener)
		return nil, err
	}
	// check this node exist
	err = ctrl.checkNodeExists(&nrr.GUID)
	if err != nil {
		return nil, err
	}
	// set action
	objects := make(map[string]interface{})
	objects["listener"] = listener
	objects["request"] = nrr
	id := ctrl.actionMgr.Store(objects, messages.MaxRegisterWaitTime)
	nnr := NoticeNodeRegister{
		ID:           id,
		GUID:         hexByteSlice(nrr.GUID[:]),
		PublicKey:    hexByteSlice(nrr.PublicKey),
		KexPublicKey: hexByteSlice(nrr.KexPublicKey),
		ConnAddress:  nrr.ConnAddress,
		SystemInfo:   nrr.SystemInfo,
		RequestTime:  nrr.RequestTime,
	}
	return &nnr, nil
}

// ConfirmTrustNode is used to confirm trust and register Node.
func (ctrl *Ctrl) ConfirmTrustNode(ctx context.Context, id string) error {
	// get objects about action, see Ctrl.TrustNode()
	object, err := ctrl.actionMgr.Load(id)
	if err != nil {
		return err
	}
	objects := object.(map[string]interface{})
	listener := objects["listener"].(*bootstrap.Listener)
	nrr := objects["request"].(*messages.NodeRegisterRequest)
	// check this node exist
	err = ctrl.checkNodeExists(&nrr.GUID)
	if err != nil {
		return err
	}
	ctrl.logger.Printf(logger.Info, "trust-node", "confirm listener: %s", listener)
	// connect node
	client, err := ctrl.NewClient(ctx, listener, nil, nil)
	if err != nil {
		return err
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
	if !bytes.Equal(reply, []byte{messages.RegisterResultAccept}) {
		return errors.Errorf("failed to trust node: %s", reply)
	}
	return nil
}

// node key exchange public key (curve25519),
// use session key encrypt register request data.
// +----------------+----------------+
// | kex public key | encrypted data |
// +----------------+----------------+
// |    32 Bytes    |       var      |
// +----------------+----------------+
func (ctrl *Ctrl) resolveNodeRegisterRequest(reply []byte) (*messages.NodeRegisterRequest, error) {
	if len(reply) < curve25519.ScalarSize+aes.BlockSize {
		return nil, errors.New("node send register request with invalid size")
	}
	// calculate node session key
	key, err := ctrl.global.KeyExchange(reply[:curve25519.ScalarSize])
	if err != nil {
		return nil, errors.Wrap(err, "failed to calculate node session key")
	}
	// decrypt node register request
	request, err := aes.CBCDecrypt(reply[curve25519.ScalarSize:], key, key[:aes.IVSize])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt node register request")
	}
	// check node register request
	nrr := messages.NodeRegisterRequest{}
	err = msgpack.Unmarshal(request, &nrr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid node register request data")
	}
	err = nrr.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "invalid node register request")
	}
	return &nrr, nil
}

func (ctrl *Ctrl) checkNodeExists(guid *guid.GUID) error {
	_, err := ctrl.database.SelectNode(guid)
	if err == nil {
		return errors.Errorf("node already exists\n%s", guid.Print())
	}
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return err
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
		ctrl.logger.Print(logger.Exploit, "register-node", err)
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

// AcceptRegisterNode is used to accept register Node.
func (ctrl *Ctrl) AcceptRegisterNode(
	nrr *messages.NodeRegisterRequest,
	listeners map[guid.GUID][]*bootstrap.Listener,
	bootstrap bool,
) error {
	// TODO add Log
	certificate, err := ctrl.registerNode(nrr, bootstrap)
	if err != nil {
		return err
	}
	// broadcast Node register response
	response := messages.NodeRegisterResponse{
		GUID:         nrr.GUID,
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		RequestTime:  nrr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultAccept,
		Certificate:  certificate.Encode(),
	}
	node, err := ctrl.database.SelectNode(&nrr.GUID)
	if err != nil {
		return err
	}
	sessionKey := node.SessionKey.Get()
	defer node.SessionKey.Put(sessionKey)
	listenersData, err := msgpack.Marshal(listeners)
	if err != nil {
		return errors.Wrap(err, "failed to marshal listeners data")
	}
	aesKey := sessionKey
	aesIV := sessionKey[:aes.IVSize]
	response.NodeListeners, err = aes.CBCEncrypt(listenersData, aesKey, aesIV)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt listeners data")
	}
	err = ctrl.sender.Broadcast(messages.CMDBNodeRegisterResponse, &response, true)
	if err != nil {
		return errors.Wrap(err, "failed to accept register node")
	}
	return nil
}

// RefuseRegisterNode is used to refuse register Node, it will call firewall.
func (ctrl *Ctrl) RefuseRegisterNode(nrr *messages.NodeRegisterRequest) error {
	response := messages.NodeRegisterResponse{
		GUID:         nrr.GUID,
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		RequestTime:  nrr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultRefused,
		// padding for Validate()
		Certificate: make([]byte, protocol.CertificateSize),
	}
	err := ctrl.sender.Broadcast(messages.CMDBNodeRegisterResponse, &response, true)
	if err != nil {
		return errors.Wrap(err, "failed to refuse register node")
	}
	return nil
}

func (ctrl *Ctrl) registerBeacon(brr *messages.BeaconRegisterRequest) error {
	failed := func(err error) error {
		return errors.Wrap(err, "failed to register beacon")
	}
	// calculate session key
	sessionKey, err := ctrl.global.KeyExchange(brr.KexPublicKey)
	if err != nil {
		err = errors.WithMessage(err, "failed to calculate session key")
		ctrl.logger.Print(logger.Exploit, "register-beacon", err)
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
func (ctrl *Ctrl) AcceptRegisterBeacon(
	brr *messages.BeaconRegisterRequest,
	listeners map[guid.GUID][]*bootstrap.Listener,
) error {
	err := ctrl.registerBeacon(brr)
	if err != nil {
		return err
	}
	// broadcast Beacon register response
	response := messages.BeaconRegisterResponse{
		GUID:         brr.GUID,
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		RequestTime:  brr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultAccept,
	}
	beacon, err := ctrl.database.SelectBeacon(&brr.GUID)
	if err != nil {
		return err
	}
	sessionKey := beacon.SessionKey.Get()
	defer beacon.SessionKey.Put(sessionKey)
	listenersData, err := msgpack.Marshal(listeners)
	if err != nil {
		return errors.Wrap(err, "failed to marshal listeners data")
	}
	aesKey := sessionKey
	aesIV := sessionKey[:aes.IVSize]
	response.NodeListeners, err = aes.CBCEncrypt(listenersData, aesKey, aesIV)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt listeners data")
	}
	err = ctrl.sender.Broadcast(messages.CMDBBeaconRegisterResponse, &response, true)
	if err != nil {
		return errors.Wrap(err, "failed to accept register beacon")
	}
	return nil
}

// RefuseRegisterBeacon is used to refuse register Beacon, it will call firewall.
func (ctrl *Ctrl) RefuseRegisterBeacon(brr *messages.BeaconRegisterRequest) error {
	response := messages.BeaconRegisterResponse{
		GUID:         brr.GUID,
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		RequestTime:  brr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultRefused,
	}
	err := ctrl.sender.Broadcast(messages.CMDBBeaconRegisterResponse, &response, true)
	if err != nil {
		return errors.Wrap(err, "failed to refuse register beacon")
	}
	return nil
}

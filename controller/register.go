package controller

import (
	"bytes"
	"context"
	"fmt"
	"strings"

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
	// store objects about action
	action := make(map[string]interface{})
	action["listener"] = listener
	action["request"] = nrr
	id := ctrl.actionMgr.Store(action, messages.MaxRegisterWaitTime)
	nnr := NoticeNodeRegister{
		ID:           id,
		GUID:         nrr.GUID,
		PublicKey:    hexByteSlice(nrr.PublicKey),
		KexPublicKey: hexByteSlice(nrr.KexPublicKey),
		ConnAddress:  nrr.ConnAddress,
		SystemInfo:   nrr.SystemInfo,
		RequestTime:  nrr.RequestTime,
	}
	return &nnr, nil
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

// ConfirmTrustNode is used to confirm trust and register Node.
func (ctrl *Ctrl) ConfirmTrustNode(ctx context.Context, reply *ReplyNodeRegister) error {
	// load objects about action, see Ctrl.TrustNode()
	action, err := ctrl.actionMgr.Load(reply.ID)
	if err != nil {
		return err
	}
	objects := action.(map[string]interface{})
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
	certificate, err := ctrl.registerNode(nrr, reply)
	if err != nil {
		return err
	}
	// send certificate
	response, err := client.send(protocol.CtrlSetNodeCert, certificate.Encode())
	if err != nil {
		return errors.WithMessage(err, "failed to set node certificate")
	}
	if !bytes.Equal(response, []byte{messages.RegisterResultAccept}) {
		return errors.Errorf("failed to trust node: %s", response)
	}
	// TODO add node listener
	// get listeners
	// response, err = client.send(protocol.CtrlQueryListeners, nil)
	// if err != nil {
	// 	return errors.WithMessage(err, "failed to set node certificate")
	// }
	// if len(response) == 0 {
	// 	return errors.New("no listener tag")
	// }
	// // add node listener
	// tag := string(response)
	// return ctrl.database.InsertNodeListener(&mNodeListener{
	// 	GUID:    nil,
	// 	Tag:     tag,
	// 	Mode:    listener.Mode,
	// 	Network: listener.Network,
	// 	Address: listener.Address,
	// })
	return nil
}

// -----------------------------------------Node register------------------------------------------

func (ctrl *Ctrl) checkNodeExists(guid *guid.GUID) error {
	_, err := ctrl.database.SelectNode(guid)
	if err == nil {
		return errors.Errorf("node already exists\n%s", guid.Print())
	}
	if err.Error() == fmt.Sprintf("node %s doesn't exist", guid.Hex()) {
		return nil
	}
	return err
}

func (ctrl *Ctrl) registerNode(
	nrr *messages.NodeRegisterRequest,
	reply *ReplyNodeRegister,
) (*protocol.Certificate, error) {
	const errMsg = "failed to register node"
	failed := func(err error) error {
		return errors.Wrap(err, errMsg)
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
		err = errors.WithMessage(err, "failed to calculate node session key")
		ctrl.logger.Print(logger.Exploit, "register-node", err)
		return nil, failed(err)
	}
	defer security.CoverBytes(sessionKey)
	// insert to database
	err = ctrl.database.InsertNode(&mNode{
		GUID:         nrr.GUID[:],
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		SessionKey:   security.NewBytes(sessionKey),
	}, &mNodeInfo{
		GUID:        nrr.GUID[:],
		IP:          strings.Join(nrr.SystemInfo.IP, ","),
		OS:          nrr.SystemInfo.OS,
		Arch:        nrr.SystemInfo.Arch,
		GoVersion:   nrr.SystemInfo.GoVersion,
		PID:         nrr.SystemInfo.PID,
		PPID:        nrr.SystemInfo.PPID,
		Hostname:    nrr.SystemInfo.Hostname,
		Username:    nrr.SystemInfo.Username,
		IsBootstrap: reply.Bootstrap,
		Zone:        reply.Zone,
	})
	if err != nil {
		return nil, errors.WithMessage(err, errMsg)
	}
	return &certificate, nil
}

// NoticeNodeRegister is used to notice user to reply Node register request.
func (ctrl *Ctrl) NoticeNodeRegister(
	nrr *messages.NodeRegisterRequest,
	node *guid.GUID,
) *NoticeNodeRegister {
	// store objects about action
	action := make(map[string]interface{})
	action["request"] = nrr
	nodeGUID := *node
	action["guid"] = &nodeGUID
	id := ctrl.actionMgr.Store(action, messages.MaxRegisterWaitTime)
	// notice view
	nnr := NoticeNodeRegister{
		ID:           id,
		GUID:         nrr.GUID,
		PublicKey:    hexByteSlice(nrr.PublicKey),
		KexPublicKey: hexByteSlice(nrr.KexPublicKey),
		ConnAddress:  nrr.ConnAddress,
		SystemInfo:   nrr.SystemInfo,
		RequestTime:  nrr.RequestTime,
	}
	return &nnr
}

// ReplyNodeRegister is used to reply Node register request.
func (ctrl *Ctrl) ReplyNodeRegister(ctx context.Context, reply *ReplyNodeRegister) error {
	// load objects about action, see Ctrl.NoticeNodeRegister()
	action, err := ctrl.actionMgr.Load(reply.ID)
	if err != nil {
		return err
	}
	objects := action.(map[string]interface{})
	nrr := objects["request"].(*messages.NodeRegisterRequest)
	nodeGUID := objects["guid"].(*guid.GUID)
	switch reply.Result {
	case messages.RegisterResultAccept:
		return ctrl.acceptRegisterNode(ctx, nrr, reply, nodeGUID)
	case messages.RegisterResultRefused:
		return ctrl.refuseRegisterNode(ctx, nrr, nodeGUID)
	}
	return fmt.Errorf("%s: %d", messages.ErrRegisterUnknownResult, reply.Result)
}

func (ctrl *Ctrl) acceptRegisterNode(
	ctx context.Context,
	nrr *messages.NodeRegisterRequest,
	reply *ReplyNodeRegister,
	guid *guid.GUID,
) error {
	err := ctrl.checkNodeExists(&nrr.GUID)
	if err != nil {
		return err
	}
	certificate, err := ctrl.registerNode(nrr, reply)
	if err != nil {
		return err
	}
	// send Node register response to the Node that forwarder this request
	response := messages.NodeRegisterResponse{
		ID:           nrr.ID,
		GUID:         nrr.GUID,
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		RequestTime:  nrr.RequestTime,
		ReplyTime:    ctrl.global.Now(),
		Result:       messages.RegisterResultAccept,
		Certificate:  certificate.Encode(),
	}
	// query Node listener and encode it.
	listeners, err := ctrl.queryNodeListener(reply.Listeners)
	if err != nil {
		return errors.Wrap(err, "failed to query node listener")
	}
	listenersData, err := msgpack.Marshal(listeners)
	if err != nil {
		return errors.Wrap(err, "failed to marshal node listeners data")
	}
	defer security.CoverBytes(listenersData)
	// encrypt listener data
	node, err := ctrl.database.SelectNode(&nrr.GUID)
	if err != nil {
		return err
	}
	sessionKey := node.SessionKey.Get()
	defer node.SessionKey.Put(sessionKey)
	aesKey := sessionKey
	aesIV := sessionKey[:aes.IVSize]
	response.NodeListeners, err = aes.CBCEncrypt(listenersData, aesKey, aesIV)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt listeners data")
	}
	// send response
	err = ctrl.sender.SendToNode(ctx, guid, messages.CMDBNodeRegisterResponse,
		&response, true)
	if err != nil {
		return errors.Wrap(err, "failed to send response to node")
	}
	return nil
}

func (ctrl *Ctrl) queryNodeListener(
	listeners map[guid.GUID][]string,
) (map[guid.GUID][]*bootstrap.Listener, error) {
	result := make(map[guid.GUID][]*bootstrap.Listener, len(listeners))
	for nodeGUID, tags := range listeners {
		listeners, err := ctrl.database.SelectNodeListener(&nodeGUID)
		if err != nil {
			return nil, err
		}
		result[nodeGUID] = selectNodeListener(listeners, tags)
	}
	return result, nil
}

func selectNodeListener(listeners []*mNodeListener, tags []string) []*bootstrap.Listener {
	var selected []*bootstrap.Listener
	for _, tag := range tags {
		for _, listener := range listeners {
			if listener.Tag == tag {
				selected = append(selected, &bootstrap.Listener{
					Mode:    listener.Mode,
					Network: listener.Network,
					Address: listener.Address,
				})
				// not break, different node listener maybe has the same tag,
				// dont't worry, Node and Beacon will not add the same listener.
			}
		}
	}
	return selected
}

func (ctrl *Ctrl) refuseRegisterNode(
	ctx context.Context,
	nrr *messages.NodeRegisterRequest,
	guid *guid.GUID,
) error {
	// first reply the Node.
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
	// send response
	err := ctrl.sender.SendToNode(ctx, guid, messages.CMDBNodeRegisterResponse,
		&response, true)
	if err != nil {
		return errors.Wrap(err, "failed to send response to node")
	}
	// then call firewall.
	return nil
}

// AcceptRegisterNode is used to accept register Node.
func (ctrl *Ctrl) AcceptRegisterNode(
	nrr *messages.NodeRegisterRequest,
	listeners map[guid.GUID][]*bootstrap.Listener,
	bootstrap bool,
) error {
	certificate, err := ctrl.registerNode(nrr, nil)
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

// ----------------------------------------Beacon register-----------------------------------------

func (ctrl *Ctrl) checkBeaconExists(guid *guid.GUID) error {
	_, err := ctrl.database.SelectBeacon(guid)
	if err == nil {
		return errors.Errorf("beacon already exists\n%s", guid.Print())
	}
	if err.Error() == fmt.Sprintf("beacon %s doesn't exist", guid.Hex()) {
		return nil
	}
	return err
}

func (ctrl *Ctrl) registerBeacon(brr *messages.BeaconRegisterRequest) error {
	const errMsg = "failed to register beacon"
	// calculate session key
	sessionKey, err := ctrl.global.KeyExchange(brr.KexPublicKey)
	if err != nil {
		err = errors.WithMessage(err, "failed to calculate beacon session key")
		ctrl.logger.Print(logger.Exploit, "register-beacon", err)
		return errors.Wrap(err, errMsg)
	}
	defer security.CoverBytes(sessionKey)
	// insert to database
	err = ctrl.database.InsertBeacon(&mBeacon{
		GUID:         brr.GUID[:],
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		SessionKey:   security.NewBytes(sessionKey),
	}, &mBeaconInfo{
		GUID:      brr.GUID[:],
		IP:        strings.Join(brr.SystemInfo.IP, ","),
		OS:        brr.SystemInfo.OS,
		Arch:      brr.SystemInfo.Arch,
		GoVersion: brr.SystemInfo.GoVersion,
		PID:       brr.SystemInfo.PID,
		PPID:      brr.SystemInfo.PPID,
		Hostname:  brr.SystemInfo.Hostname,
		Username:  brr.SystemInfo.Username,
	})
	if err != nil {
		return errors.WithMessage(err, errMsg)
	}
	return nil
}

// NoticeBeaconRegister is used to notice user to reply Beacon register request.
func (ctrl *Ctrl) NoticeBeaconRegister(
	brr *messages.BeaconRegisterRequest,
	node *guid.GUID,
) *NoticeBeaconRegister {
	// store objects about action
	action := make(map[string]interface{})
	action["request"] = brr
	nodeGUID := *node
	action["guid"] = &nodeGUID
	id := ctrl.actionMgr.Store(action, messages.MaxRegisterWaitTime)
	// notice view
	nbr := NoticeBeaconRegister{
		ID:           id,
		GUID:         brr.GUID,
		PublicKey:    hexByteSlice(brr.PublicKey),
		KexPublicKey: hexByteSlice(brr.KexPublicKey),
		ConnAddress:  brr.ConnAddress,
		SystemInfo:   brr.SystemInfo,
		RequestTime:  brr.RequestTime,
	}
	return &nbr
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

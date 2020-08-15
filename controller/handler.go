package controller

import (
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"hash"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/patch/msgpack"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xpanic"
)

type handler struct {
	ctx *Ctrl

	rand *random.Rand

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newHandler(ctx *Ctrl) *handler {
	h := handler{
		ctx:  ctx,
		rand: random.NewRand(),
	}
	h.context, h.cancel = context.WithCancel(context.Background())
	return &h
}

func (h *handler) Cancel() {
	h.cancel()
}

func (h *handler) Close() {
	h.wg.Wait()
	h.ctx = nil
}

func (h *handler) logf(lv logger.Level, format string, log ...interface{}) {
	h.ctx.logger.Printf(lv, "handler", format, log...)
}

func (h *handler) log(lv logger.Level, log ...interface{}) {
	h.ctx.logger.Println(lv, "handler", log...)
}

// logPanic must use like defer h.logPanic("title")
func (h *handler) logPanic(title string) {
	if r := recover(); r != nil {
		h.log(logger.Fatal, xpanic.Print(r, title))
	}
}

// logfWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo logf
// GUID: FF...
//       FF...
// spew output...
//
// first log interface must be role GUID
// second log interface must be *protocol.Send
func (h *handler) logfWithInfo(lv logger.Level, format string, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, format, log[2:]...)
	_, _ = fmt.Fprintf(buf, "\n%s\n", log[0].(*guid.GUID).Print())
	spew.Fdump(buf, log[1])
	h.ctx.logger.Print(lv, "handler", buf)
}

// logWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo log
// GUID: FF...
//       FF...
// spew output...
//
// first log interface must be role GUID
// second log interface must be *protocol.Send
func (h *handler) logWithInfo(lv logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log[2:]...)
	_, _ = fmt.Fprintf(buf, "%s\n", log[0].(*guid.GUID).Print())
	spew.Fdump(buf, log[1])
	h.ctx.logger.Print(lv, "handler", buf)
}

// ----------------------------------------Node Send-----------------------------------------------

func (h *handler) OnNodeSend(send *protocol.Send) {
	defer h.logPanic("handler.OnNodeSend")
	if len(send.Message) < messages.HeaderSize {
		h.logWithInfo(logger.Exploit, send.RoleGUID, send, "node send with invalid size")
		return
	}
	typ := send.Message[messages.RandomDataSize:messages.HeaderSize]
	msgType := convert.BEBytesToUint32(typ)
	send.Message = send.Message[messages.HeaderSize:]
	switch msgType {
	case messages.CMDNodeLog:
		h.handleNodeLog(send)
	case messages.CMDNodeQueryNodeKey:
		h.handleQueryNodeKey(send)
	case messages.CMDNodeQueryBeaconKey:
		h.handleQueryBeaconKey(send)
	case messages.CMDNodeUpdateNodeRequestFromNode:
		h.handleNodeUpdateNodeRequest(send)
	case messages.CMDNodeUpdateNodeRequestFromBeacon:
		h.handleBeaconUpdateNodeRequest(send)
	case messages.CMDNodeRegisterRequestFromNode:
		h.handleNodeRegisterRequest(send)
	case messages.CMDNodeRegisterRequestFromBeacon:
		h.handleBeaconRegisterRequest(send)
	case messages.CMDTest:
		h.handleNodeSendTestMessage(send)
	case messages.CMDRTTestRequest:
		h.handleNodeSendTestRequest(send)
	case messages.CMDRTTestResponse:
		h.handleNodeSendTestResponse(send)
	default:
		const format = "node send unknown message\n%s\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, send.RoleGUID.Print(), msgType, spew.Sdump(send))
	}
}

func (h *handler) handleNodeLog(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeLog")
	log := messages.Log{}
	err := msgpack.Unmarshal(send.Message, &log)
	if err != nil {
		const format = "node send invalid log data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	err = h.ctx.database.InsertNodeLog(&mRoleLog{
		GUID:      send.RoleGUID[:],
		CreatedAt: log.Time,
		Level:     log.Level,
		Source:    log.Source,
		Log:       log.Log,
	})
	if err != nil {
		const format = "failed to insert node log\nerror: %s"
		h.logfWithInfo(logger.Error, format, &send.RoleGUID, send, err)
	}
}

// -------------------------------------query role key---------------------------------------------

func (h *handler) handleQueryNodeKey(send *protocol.Send) {
	defer h.logPanic("handler.handleQueryNodeKey")
	qnk := messages.QueryNodeKey{}
	err := msgpack.Unmarshal(send.Message, &qnk)
	if err != nil {
		const format = "node send invalid query node key data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	ank := messages.AnswerNodeKey{
		ID: qnk.ID,
	}
	node, err := h.ctx.database.SelectNode(&qnk.GUID)
	if err != nil {
		const format = "failed to query node key\nerror: %s"
		h.logfWithInfo(logger.Warning, format, &send.RoleGUID, &qnk, err)
		// padding
		ank.PublicKey = messages.ZeroPublicKey
		ank.KexPublicKey = messages.ZeroKexPublicKey
	} else {
		ank.GUID = qnk.GUID
		ank.PublicKey = node.PublicKey
		ank.KexPublicKey = node.KexPublicKey
		ank.ReplyTime = node.CreatedAt
	}
	// send to Node
	err = h.ctx.sender.SendToNode(h.context, &send.RoleGUID, messages.CMDBCtrlAnswerNodeKey,
		&ank, true)
	if err != nil {
		const format = "failed to answer node key\nerror: %s"
		h.logfWithInfo(logger.Error, format, &send.RoleGUID, &ank, err)
		return
	}
	const format = "node query node key\n%s"
	h.logfWithInfo(logger.Info, format, &send.RoleGUID, nil, qnk.GUID.Print())
}

func (h *handler) handleQueryBeaconKey(send *protocol.Send) {
	defer h.logPanic("handler.handleQueryBeaconKey")
	qbk := messages.QueryBeaconKey{}
	err := msgpack.Unmarshal(send.Message, &qbk)
	if err != nil {
		const format = "node send invalid query beacon key data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	abk := messages.AnswerBeaconKey{
		ID: qbk.ID,
	}
	beacon, err := h.ctx.database.SelectBeacon(&qbk.GUID)
	if err != nil {
		const format = "failed to query beacon key\nerror: %s"
		h.logfWithInfo(logger.Warning, format, &send.RoleGUID, &qbk, err)
		// padding
		abk.PublicKey = messages.ZeroPublicKey
		abk.KexPublicKey = messages.ZeroKexPublicKey
	} else {
		abk.GUID = qbk.GUID
		abk.PublicKey = beacon.PublicKey
		abk.KexPublicKey = beacon.KexPublicKey
		abk.ReplyTime = beacon.CreatedAt
	}
	// send to Node
	err = h.ctx.sender.SendToNode(h.context, &send.RoleGUID, messages.CMDBCtrlAnswerBeaconKey,
		&abk, true)
	if err != nil {
		const format = "failed to answer beacon key\nerror: %s"
		h.logfWithInfo(logger.Error, format, &send.RoleGUID, &abk, err)
		return
	}
	const format = "node query beacon key\n%s"
	h.logfWithInfo(logger.Info, format, &send.RoleGUID, nil, qbk.GUID.Print())
}

// ---------------------------------role update node request---------------------------------------

func (h *handler) handleNodeUpdateNodeRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeUpdateNodeRequest")
	unr := h.resolveUpdateNodeRequest(protocol.Node, send)
	if unr == nil {
		return
	}
	// send to Node
	err := h.ctx.sender.SendToNode(h.context, &send.RoleGUID, messages.CMDBCtrlUpdateNodeResponse,
		unr, true)
	if err != nil {
		const log = "failed to send update node response from node\nerror:"
		h.logWithInfo(logger.Error, log, &send.RoleGUID, &unr, err)
	}
}

func (h *handler) handleBeaconUpdateNodeRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconUpdateNodeRequest")
	unr := h.resolveUpdateNodeRequest(protocol.Beacon, send)
	if unr == nil {
		return
	}
	// send to Node
	err := h.ctx.sender.SendToNode(h.context, &send.RoleGUID, messages.CMDBCtrlUpdateNodeResponse,
		unr, true)
	if err != nil {
		const log = "failed to send update node response from beacon\nerror:"
		h.logWithInfo(logger.Error, log, &send.RoleGUID, &unr, err)
	}
}

func (h *handler) resolveUpdateNodeRequest(
	role protocol.Role,
	send *protocol.Send,
) *messages.UpdateNodeResponse {
	defer h.logPanic("handler.decryptUpdateNodeRequest")
	mUNR := messages.UpdateNodeRequest{}
	err := msgpack.Unmarshal(send.Message, &mUNR)
	if err != nil {
		const format = "invalid %s update node request data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil
	}
	response := messages.UpdateNodeResponse{
		ID: mUNR.ID,
	}
	pUNReq := protocol.NewUpdateNodeRequest()
	err = pUNReq.Unpack(mUNR.Data)
	if err != nil {
		const format = "invalid %s update node request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil
	}
	err = h.checkUpdateNodeRequest(role, pUNReq)
	if err != nil {
		const format = "failed to check %s update node request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil
	}
	// check node is exist
	nodeGUID := guid.GUID{}
	copy(nodeGUID[:], pUNReq.EncData[:guid.Size])
	publicKey := pUNReq.EncData[guid.Size:]
	// random data, only the first bytes is useful
	resp := h.rand.Bytes(8)
	node, err := h.ctx.database.SelectNode(&nodeGUID)
	if err != nil {
		resp[0] = protocol.UpdateNodeResponseNotExist
	} else {
		if bytes.Equal(node.PublicKey, publicKey) {
			resp[0] = protocol.UpdateNodeResponseOK
		} else {
			// <security> role connect a fake Node(with incorrect public key)
			resp[0] = protocol.UpdateNodeResponseIncorrectPublicKey
			const format = "%s %s connect fake node %s"
			h.logf(logger.Exploit, format, role, pUNReq.GUID.Hex(), nodeGUID.Hex())
		}
	}
	// pack response
	encResp, err := h.encryptUpdateNodeResponse(role, pUNReq, resp)
	if err != nil {
		const format = "failed to encrypt %s update node response\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil
	}
	buf := bytes.NewBuffer(make([]byte, 0, aes.BlockSize))
	encResp.Pack(buf)
	response.Data = buf.Bytes()
	return &response
}

func (h *handler) checkUpdateNodeRequest(role protocol.Role, r *protocol.UpdateNodeRequest) error {
	var (
		sessionKey *security.Bytes
		hmacPool   *sync.Pool
	)
	switch role {
	case protocol.Node:
		node, err := h.ctx.database.SelectNode(&r.GUID)
		if err != nil {
			return err
		}
		sessionKey = node.SessionKey
		hmacPool = &node.HMACPool
	case protocol.Beacon:
		beacon, err := h.ctx.database.SelectBeacon(&r.GUID)
		if err != nil {
			return err
		}
		sessionKey = beacon.SessionKey
		hmacPool = &beacon.HMACPool
	default:
		panic(fmt.Sprintf("handler: invalid role: %s", role))
	}
	sk := sessionKey.Get()
	defer sessionKey.Put(sk)
	hmac := hmacPool.Get().(hash.Hash)
	defer hmacPool.Put(hmac)
	// decrypt
	var err error
	r.EncData, err = aes.CBCDecrypt(r.EncData, sk, sk[:aes.IVSize])
	if err != nil {
		return err
	}
	// check hash
	hmac.Reset()
	hmac.Write(r.GUID[:])
	hmac.Write(r.EncData)
	if subtle.ConstantTimeCompare(hmac.Sum(nil), r.Hash) != 1 {
		return errors.New("incorrect hash")
	}
	return nil
}

func (h *handler) encryptUpdateNodeResponse(
	role protocol.Role,
	unr *protocol.UpdateNodeRequest,
	response []byte,
) (*protocol.UpdateNodeResponse, error) {
	var (
		sessionKey *security.Bytes
		hmacPool   *sync.Pool
	)
	switch role {
	case protocol.Node:
		node, err := h.ctx.database.SelectNode(&unr.GUID)
		if err != nil {
			return nil, err
		}
		sessionKey = node.SessionKey
		hmacPool = &node.HMACPool
	case protocol.Beacon:
		beacon, err := h.ctx.database.SelectBeacon(&unr.GUID)
		if err != nil {
			return nil, err
		}
		sessionKey = beacon.SessionKey
		hmacPool = &beacon.HMACPool
	default:
		panic(fmt.Sprintf("handler: invalid role: %s", role))
	}
	sk := sessionKey.Get()
	defer sessionKey.Put(sk)
	hmac := hmacPool.Get().(hash.Hash)
	defer hmacPool.Put(hmac)
	pUNResp := protocol.UpdateNodeResponse{}
	// encrypt
	var err error
	pUNResp.EncData, err = aes.CBCEncrypt(response, sk, sk[:aes.IVSize])
	if err != nil {
		panic(err)
	}
	// calculate hash
	hmac.Reset()
	hmac.Write(unr.GUID[:])
	hmac.Write(pUNResp.EncData)
	pUNResp.Hash = hmac.Sum(nil)
	return &pUNResp, nil
}

// ----------------------------------role register request-----------------------------------------

func (h *handler) handleNodeRegisterRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeRegisterRequest")
	encRR, request := h.decryptRegisterRequest(protocol.Node, send)
	if len(request) == 0 {
		return
	}
	nrr := messages.NodeRegisterRequest{}
	err := msgpack.Unmarshal(request, &nrr)
	if err != nil {
		const format = "node send invalid node register request data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, request, err)
		return
	}
	err = nrr.Validate()
	if err != nil {
		const format = "node send invalid node register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, request, err)
		return
	}
	// compare key exchange public key
	if !bytes.Equal(encRR.KexPublicKey, nrr.KexPublicKey) {
		const log = "different key exchange public key in node register request"
		h.logWithInfo(logger.Exploit, &send.RoleGUID, send, log)
		return
	}
	nnr := h.ctx.NoticeNodeRegister(&send.RoleGUID, &encRR.ID, &nrr)
	h.ctx.Test.AddNoticeNodeRegister(h.context, nnr)
}

func (h *handler) handleBeaconRegisterRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconRegisterRequest")
	encRR, request := h.decryptRegisterRequest(protocol.Beacon, send)
	if len(request) == 0 {
		return
	}
	brr := messages.BeaconRegisterRequest{}
	err := msgpack.Unmarshal(request, &brr)
	if err != nil {
		const format = "node send invalid beacon register request data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, request, err)
		return
	}
	err = brr.Validate()
	if err != nil {
		const format = "node send invalid beacon register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, request, err)
		return
	}
	// compare key exchange public key
	if !bytes.Equal(encRR.KexPublicKey, brr.KexPublicKey) {
		const log = "different key exchange public key in beacon register request"
		h.logWithInfo(logger.Exploit, &send.RoleGUID, send, log)
		return
	}
	nbr := h.ctx.NoticeBeaconRegister(&send.RoleGUID, &encRR.ID, &brr)
	h.ctx.Test.AddNoticeBeaconRegister(h.context, nbr)
}

func (h *handler) decryptRegisterRequest(
	role protocol.Role,
	send *protocol.Send,
) (*messages.EncryptedRegisterRequest, []byte) {
	defer h.logPanic("handler.decryptRegisterRequest")
	encRR := messages.EncryptedRegisterRequest{}
	err := msgpack.Unmarshal(send.Message, &encRR)
	if err != nil {
		const format = "node send invalid encrypted %s register request data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil, nil
	}
	err = encRR.Validate()
	if err != nil {
		const format = "node send invalid encrypted %s register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil, nil
	}
	// calculate role session key
	key, err := h.ctx.global.KeyExchange(encRR.KexPublicKey)
	if err != nil {
		const format = "node send invalid %s register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil, nil
	}
	// decrypt role register request
	request, err := aes.CBCDecrypt(encRR.EncRequest, key, key[:aes.IVSize])
	if err != nil {
		const format = "node send invalid %s register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, role, err)
		return nil, nil
	}
	return &encRR, request
}

// ----------------------------------------send test-----------------------------------------------

func (h *handler) handleNodeSendTestMessage(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeSendTestMessage")
	err := h.ctx.Test.AddNodeSendMessage(h.context, &send.RoleGUID, send.Message)
	if err != nil {
		const log = "failed to add node send test message\nerror:"
		h.logWithInfo(logger.Fatal, &send.RoleGUID, send, log, err)
	}
}

func (h *handler) handleNodeSendTestRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeSendTestRequest")
	request := messages.TestRequest{}
	err := msgpack.Unmarshal(send.Message, &request)
	if err != nil {
		const format = "invalid node test request data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	// send response
	response := messages.TestResponse{
		ID:       request.ID,
		Response: request.Request,
	}
	err = h.ctx.sender.SendToNode(h.context, &send.RoleGUID,
		messages.CMDBRTTestResponse, &response, true)
	if err != nil {
		const format = "failed to send node test response to node\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
	}
}

func (h *handler) handleNodeSendTestResponse(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeSendTestResponse")
	response := messages.TestResponse{}
	err := msgpack.Unmarshal(send.Message, &response)
	if err != nil {
		const format = "invalid node test response data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	h.ctx.messageMgr.HandleNodeReply(&send.RoleGUID, &response.ID, &response)
}

// ---------------------------------------Beacon Send----------------------------------------------

func (h *handler) OnBeaconSend(send *protocol.Send) {
	defer h.logPanic("handler.OnBeaconSend")
	if len(send.Message) < messages.HeaderSize {
		h.logWithInfo(logger.Exploit, send.RoleGUID, send, "beacon send with invalid size")
		return
	}
	typ := send.Message[messages.RandomDataSize:messages.HeaderSize]
	msgType := convert.BEBytesToUint32(typ)
	send.Message = send.Message[messages.HeaderSize:]
	switch msgType {
	case messages.CMDShellCodeResult:
		h.handleShellCodeResult(send)
	case messages.CMDSingleShellOutput:
		h.handleSingleShellOutput(send)
	case messages.CMDBeaconModeChanged:
		h.handleBeaconModeChanged(send)
	case messages.CMDBeaconLog:
		h.handleBeaconLog(send)
	case messages.CMDTest:
		h.handleBeaconSendTestMessage(send)
	case messages.CMDRTTestRequest:
		h.handleBeaconSendTestRequest(send)
	case messages.CMDRTTestResponse:
		h.handleBeaconSendTestResponse(send)
	default:
		const format = "beacon send unknown message\n%s\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, send.RoleGUID.Print(), msgType, spew.Sdump(send))
	}
}

func (h *handler) handleShellCodeResult(send *protocol.Send) {
	defer h.logPanic("handler.handleShellCodeResult")
	result := messages.ShellCodeResult{}
	err := msgpack.Unmarshal(send.Message, &result)
	if err != nil {
		const format = "invalid shellcode result data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	err = h.ctx.database.InsertShellCodeResult(&send.RoleGUID, result.Err)
	if err != nil {
		const log = "failed to insert execute shellcode result\nerror:"
		h.logWithInfo(logger.Error, &send.RoleGUID, send, log, err)
		return
	}
	h.ctx.messageMgr.HandleBeaconReply(&send.RoleGUID, &result.ID, &result)
	// notice
}

func (h *handler) handleSingleShellOutput(send *protocol.Send) {
	defer h.logPanic("handler.handleSingleShellOutput")
	output := messages.SingleShellOutput{}
	err := msgpack.Unmarshal(send.Message, &output)
	if err != nil {
		const format = "invalid single shell output data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	err = h.ctx.database.InsertSingleShellOutput(&send.RoleGUID, &output)
	if err != nil {
		const log = "failed to insert single shell output\nerror:"
		h.logWithInfo(logger.Error, &send.RoleGUID, send, log, err)
		return
	}
	h.ctx.messageMgr.HandleBeaconReply(&send.RoleGUID, &output.ID, &output)
	// notice
	fmt.Println(string(output.Output))
}

func (h *handler) handleBeaconModeChanged(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconModeChanged")
	mc := messages.ModeChanged{}
	err := msgpack.Unmarshal(send.Message, &mc)
	if err != nil {
		const format = "invalid beacon mode changed data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	err = h.ctx.database.InsertBeaconModeChanged(&send.RoleGUID, &mc)
	if err != nil {
		const log = "failed to insert beacon mode changed log\nerror:"
		h.logWithInfo(logger.Error, &send.RoleGUID, send, log, err)
		return
	}
	if mc.Interactive {
		h.ctx.sender.EnableInteractiveMode(&send.RoleGUID)
	} else {
		h.ctx.sender.DisableInteractiveMode(&send.RoleGUID)
	}
	// notice
	fmt.Println(mc.Interactive, mc.Reason)
}

func (h *handler) handleBeaconLog(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconLog")
	log := messages.Log{}
	err := msgpack.Unmarshal(send.Message, &log)
	if err != nil {
		const format = "beacon send invalid log data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	err = h.ctx.database.InsertBeaconLog(&mRoleLog{
		GUID:      send.RoleGUID[:],
		CreatedAt: log.Time,
		Level:     log.Level,
		Source:    log.Source,
		Log:       log.Log,
	})
	if err != nil {
		const format = "failed to insert node log\nerror: %s"
		h.logfWithInfo(logger.Error, format, &send.RoleGUID, send, err)
	}
}

// -----------------------------------------send test----------------------------------------------

func (h *handler) handleBeaconSendTestMessage(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconSendTestMessage")
	err := h.ctx.Test.AddBeaconSendMessage(h.context, &send.RoleGUID, send.Message)
	if err != nil {
		const log = "failed to add beacon send test message\nerror:"
		h.logWithInfo(logger.Fatal, &send.RoleGUID, send, log, err)
	}
}

func (h *handler) handleBeaconSendTestRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconSendTestRequest")
	request := messages.TestRequest{}
	err := msgpack.Unmarshal(send.Message, &request)
	if err != nil {
		const format = "invalid beacon test request data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	// send response
	response := messages.TestResponse{
		ID:       request.ID,
		Response: request.Request,
	}
	err = h.ctx.sender.SendToBeacon(h.context, &send.RoleGUID,
		messages.CMDBRTTestResponse, &response, true)
	if err != nil {
		const format = "failed to send beacon test response to node\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
	}
}

func (h *handler) handleBeaconSendTestResponse(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconSendTestResponse")
	response := messages.TestResponse{}
	err := msgpack.Unmarshal(send.Message, &response)
	if err != nil {
		const format = "invalid beacon test response data\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, &send.RoleGUID, send, err)
		return
	}
	h.ctx.messageMgr.HandleBeaconReply(&send.RoleGUID, &response.ID, &response)
}

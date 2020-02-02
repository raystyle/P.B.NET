package controller

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/axgle/mahonia"
	"github.com/davecgh/go-spew/spew"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type handler struct {
	ctx *CTRL

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newHandler(ctx *CTRL) *handler {
	h := handler{
		ctx: ctx,
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

func (h *handler) logf(l logger.Level, format string, log ...interface{}) {
	h.ctx.logger.Printf(l, "handler", format, log...)
}

func (h *handler) log(l logger.Level, log ...interface{}) {
	h.ctx.logger.Println(l, "handler", log...)
}

// logfWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo logf
// GUID: FF...
// spew output
//
// first log interface must be role GUID
// second log interface must be *protocol.Send
func (h *handler) logfWithInfo(l logger.Level, format string, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, format, log[2:]...)
	g := log[0].([]byte)
	_, _ = fmt.Fprintf(buf, "\nGUID: %X\n", g[:guid.Size/2])
	_, _ = fmt.Fprintf(buf, "      %X\n", g[guid.Size/2:])
	spew.Fdump(buf, log[1])
	h.ctx.logger.Print(l, "handler", buf)
}

// logWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo log
// GUID: FF...
// spew output
//
// first log interface must be role GUID
// second log interface must be *protocol.Send
func (h *handler) logWithInfo(l logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log[2:]...)
	g := log[0].([]byte)
	_, _ = fmt.Fprintf(buf, "GUID: %X\n", g[:guid.Size/2])
	_, _ = fmt.Fprintf(buf, "      %X\n", g[guid.Size/2:])
	spew.Fdump(buf, log[1])
	h.ctx.logger.Print(l, "handler", buf)
}

// logPanic must use like defer h.logPanic("title")
func (h *handler) logPanic(title string) {
	if r := recover(); r != nil {
		h.log(logger.Fatal, xpanic.Print(r, title))
	}
}

// ----------------------------------------Node Send-----------------------------------------------

func (h *handler) OnNodeSend(send *protocol.Send) {
	defer h.logPanic("handler.OnNodeSend")
	if len(send.Message) < 4 {
		const log = "node send with invalid size"
		h.logWithInfo(logger.Exploit, send.RoleGUID, send, log)
		return
	}
	msgType := convert.BytesToUint32(send.Message[:4])
	send.Message = send.Message[4:]
	switch msgType {
	case messages.CMDNodeRegisterRequest:
		h.handleNodeRegisterRequest(send)
	case messages.CMDBeaconRegisterRequest:
		h.handleBeaconRegisterRequest(send)
	case messages.CMDShellOutput:
		h.handleNodeShellOutput(send)
	case messages.CMDTest:
		h.handleNodeSendTestMessage(send)
	default:
		buf := new(bytes.Buffer)
		_, _ = fmt.Fprintf(buf, "GUID: %X\n", send.RoleGUID[:guid.Size/2])
		_, _ = fmt.Fprintf(buf, "      %X", send.RoleGUID[guid.Size/2:])
		const format = "node send unknown message\n%s\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, buf, msgType, spew.Sdump(send))
	}
}

// ----------------------------------role register request-----------------------------------------

func (h *handler) handleNodeRegisterRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeRegisterRequest")
	request := h.decryptRoleRegisterRequest(protocol.Node, send)
	if len(request) == 0 {
		return
	}
	var nrr messages.NodeRegisterRequest
	err := msgpack.Unmarshal(request, &nrr)
	if err != nil {
		const format = "node send invalid node register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, send.RoleGUID, request, err)
		return
	}
	// compare key exchange public key
	if bytes.Compare(send.Message[:curve25519.ScalarSize], nrr.KexPublicKey) != 0 {
		const log = "different key exchange public key in node register request"
		h.logWithInfo(logger.Exploit, send.RoleGUID, send, log)
		return
	}
	// notice view

	// test
	if h.ctx.Test.NodeRegisterRequest == nil {
		return
	}
	select {
	case h.ctx.Test.NodeRegisterRequest <- &nrr:
	case <-h.context.Done():
	}
}

func (h *handler) handleBeaconRegisterRequest(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconRegisterRequest")
	request := h.decryptRoleRegisterRequest(protocol.Beacon, send)
	if len(request) == 0 {
		return
	}
	var brr messages.BeaconRegisterRequest
	err := msgpack.Unmarshal(request, &brr)
	if err != nil {
		const log = "node send invalid beacon register request"
		h.logWithInfo(logger.Exploit, send.RoleGUID, request, log)
		return
	}
	// compare key exchange public key
	if bytes.Compare(send.Message[:curve25519.ScalarSize], brr.KexPublicKey) != 0 {
		const log = "different key exchange public key in beacon register request"
		h.logWithInfo(logger.Exploit, send.RoleGUID, send, log)
		return
	}
	// notice view

	// test
	if h.ctx.Test.BeaconRegisterRequest == nil {
		return
	}
	select {
	case h.ctx.Test.BeaconRegisterRequest <- &brr:
	case <-h.context.Done():
	}
}

func (h *handler) decryptRoleRegisterRequest(role protocol.Role, send *protocol.Send) []byte {
	defer h.logPanic("handler.decryptRoleRegisterRequest")
	req := send.Message
	if len(req) < net.IPv6len+curve25519.ScalarSize+aes.BlockSize {
		const format = "node send %s register request with invalid size"
		h.logfWithInfo(logger.Exploit, format, send.RoleGUID, send, role)
		return nil
	}

	// calculate role session key
	key, err := h.ctx.global.KeyExchange(req[:curve25519.ScalarSize])
	if err != nil {
		const format = "node send invalid %s register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, send.RoleGUID, send, role, err)
		return nil
	}
	// decrypt role register request
	request, err := aes.CBCDecrypt(req[curve25519.ScalarSize:], key, key[:aes.IVSize])
	if err != nil {
		const format = "node send invalid %s register request\nerror: %s"
		h.logfWithInfo(logger.Exploit, format, send.RoleGUID, send, role, err)
		return nil
	}
	return request
}

func (h *handler) handleNodeShellOutput(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeShellOutput")
	var so messages.ShellOutput
	err := msgpack.Unmarshal(send.Message, &so)
	if err != nil {
		const log = "node send invalid shell output"
		h.logWithInfo(logger.Exploit, send.RoleGUID, send, log)
		return
	}
	str := mahonia.NewDecoder("GBK").ConvertString(string(so.Output))
	fmt.Println("controller receive it!")
	fmt.Println(str)
}

// ----------------------------------------send test-----------------------------------------------

func (h *handler) handleNodeSendTestMessage(send *protocol.Send) {
	defer h.logPanic("handler.handleNodeSendTestMessage")
	if !h.ctx.Test.roleSendTestMsgEnabled {
		return
	}
	err := h.ctx.Test.AddNodeSendTestMessage(h.context, send.RoleGUID, send.Message)
	if err != nil {
		const log = "failed to add node send test message\nerror:"
		h.logWithInfo(logger.Fatal, send.RoleGUID, send, log, err)
	}
}

// ---------------------------------------Beacon Send----------------------------------------------

func (h *handler) OnBeaconSend(send *protocol.Send) {
	defer h.logPanic("handler.OnBeaconSend")
	if len(send.Message) < 4 {
		const log = "beacon send with invalid size"
		h.logWithInfo(logger.Exploit, send.RoleGUID, send, log)
		return
	}
	msgType := convert.BytesToUint32(send.Message[:4])
	send.Message = send.Message[4:]
	switch msgType {
	case messages.CMDTest:
		h.handleBeaconSendTestMessage(send)
	default:
		buf := new(bytes.Buffer)
		_, _ = fmt.Fprintf(buf, "GUID: %X\n", send.RoleGUID[:guid.Size/2])
		_, _ = fmt.Fprintf(buf, "      %X", send.RoleGUID[guid.Size/2:])
		const format = "beacon send unknown message\n%s\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, buf, msgType, spew.Sdump(send))
	}
}

// -----------------------------------------send test----------------------------------------------

func (h *handler) handleBeaconSendTestMessage(send *protocol.Send) {
	defer h.logPanic("handler.handleBeaconSendTestMessage")
	if !h.ctx.Test.roleSendTestMsgEnabled {
		return
	}
	err := h.ctx.Test.AddBeaconSendTestMessage(h.context, send.RoleGUID, send.Message)
	if err != nil {
		const log = "failed to add beacon send test message\nerror:"
		h.logWithInfo(logger.Fatal, send.RoleGUID, send, log, err)
	}
}

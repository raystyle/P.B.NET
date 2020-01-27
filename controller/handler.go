package controller

import (
	"context"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type handler struct {
	ctx *CTRL

	wg      sync.WaitGroup
	context context.Context
	cancel  context.CancelFunc
}

func newHandler(ctx *CTRL) *handler {
	h := handler{
		ctx: ctx,
	}
	h.context, h.cancel = context.WithCancel(context.Background())
	return &h
}

func (h *handler) logf(l logger.Level, format string, log ...interface{}) {
	h.ctx.logger.Printf(l, "handler", format, log...)
}

func (h *handler) log(l logger.Level, log ...interface{}) {
	h.ctx.logger.Println(l, "handler", log...)
}

func (h *handler) Cancel() {
	h.cancel()
}

func (h *handler) Close() {
	h.ctx = nil
}

// ----------------------------------------Node Send-----------------------------------------------

func (h *handler) OnNodeSend(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.OnNodeSend")
			h.log(logger.Fatal, err)
		}
	}()
	if len(send.Message) < 4 {
		h.logf(logger.Exploit, "node %X send with invalid size", send.RoleGUID)
		return
	}
	switch convert.BytesToUint32(send.Message[:4]) {
	case messages.CMDNodeRegisterRequest:
		h.handleNodeRegisterRequest(send)
	case messages.CMDBeaconRegisterRequest:
		h.handleBeaconRegisterRequest(send)
	case messages.CMDTest:
		h.handleNodeSendTestMessage(send)
	default:
		h.logf(logger.Exploit, "node %X send unknown message", send.RoleGUID)
	}
}

func (h *handler) handleNodeRegisterRequest(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleNodeRegisterRequest")
			h.log(logger.Fatal, err)
		}
	}()
	var nrr messages.NodeRegisterRequest
	err := msgpack.Unmarshal(send.Message[4:], &nrr)
	if err != nil {
		const format = "node %X send invalid node register request: %X"
		h.logf(logger.Exploit, format, send.RoleGUID, send.Message[4:])
		return
	}
	spew.Dump(nrr)
}

func (h *handler) handleBeaconRegisterRequest(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleBeaconRegisterRequest")
			h.log(logger.Fatal, err)
		}
	}()
	var brr messages.BeaconRegisterRequest
	err := msgpack.Unmarshal(send.Message[4:], &brr)
	if err != nil {
		const format = "node %X send invalid beacon register request: %X"
		h.logf(logger.Exploit, format, send.RoleGUID, send.Message[4:])
		return
	}
	spew.Dump(brr)
}

func (h *handler) handleNodeSendTestMessage(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleNodeSendTestMessage")
			h.log(logger.Fatal, err)
		}
	}()
	if h.ctx.Test.NodeSend == nil {
		return
	}
	var testMsg []byte
	err := msgpack.Unmarshal(send.Message[4:], &testMsg)
	if err != nil {
		const format = "node %X send invalid test message: %X"
		h.logf(logger.Exploit, format, send.RoleGUID, send.Message[4:])
		return
	}
	select {
	case h.ctx.Test.NodeSend <- testMsg:
	case <-h.context.Done():
	}
}

// ---------------------------------------Beacon Send----------------------------------------------

func (h *handler) OnBeaconSend(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.OnBeaconSend")
			h.log(logger.Fatal, err)
		}
	}()
	if len(send.Message) < 4 {
		h.logf(logger.Exploit, "beacon %X send message with invalid size", send.RoleGUID)
		return
	}
	switch convert.BytesToUint32(send.Message[:4]) {
	case messages.CMDTest:
		h.handleBeaconSendTestMessage(send)
	default:
		h.logf(logger.Exploit, "beacon %X send unknown message", send.RoleGUID)
	}
}

func (h *handler) handleBeaconSendTestMessage(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleBeaconSendTestMessage")
			h.log(logger.Fatal, err)
		}
	}()
	if h.ctx.Test.BeaconSend == nil {
		return
	}
	var testMsg []byte
	err := msgpack.Unmarshal(send.Message[4:], &testMsg)
	if err != nil {
		const format = "beacon %X send invalid test message: %X"
		h.logf(logger.Exploit, format, send.RoleGUID, send.Message[4:])
		return
	}
	select {
	case h.ctx.Test.NodeSend <- testMsg:
	case <-h.context.Done():
	}
}

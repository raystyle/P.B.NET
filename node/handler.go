package node

import (
	"context"

	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type handler struct {
	ctx *Node

	context context.Context
	cancel  context.CancelFunc
}

func newHandler(ctx *Node) *handler {
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
	h.ctx.logger.Print(l, "handler", log...)
}

func (h *handler) Close() {
	h.cancel()
}

// -------------------------------------------send---------------------------------------------------

func (h *handler) OnSend(s *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.OnSend")
			h.log(logger.Fatal, err)
		}
	}()
	if len(s.Message) < 4 {
		h.logf(logger.Exploit, "controller send with invalid size")
		return
	}
	switch convert.BytesToUint32(s.Message[:4]) {
	case messages.CMDTest:
		h.handleSendTestMessage(s.Message[4:])
	default:
		h.logf(logger.Exploit, "controller send unknown message: %X", s.Message)
	}
}

func (h *handler) handleSendTestMessage(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleSendTestMessage")
			h.log(logger.Fatal, err)
		}
	}()
	if h.ctx.Test.SendTestMsg == nil {
		return
	}
	var testMsg []byte
	err := msgpack.Unmarshal(message, &testMsg)
	if err != nil {
		const format = "controller send invalid test message: %X"
		h.logf(logger.Exploit, format, message)
		return
	}
	select {
	case h.ctx.Test.SendTestMsg <- testMsg:
	case <-h.context.Done():
		return
	}
}

// ----------------------------------------broadcast-------------------------------------------------

func (h *handler) OnBroadcast(s *protocol.Broadcast) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.OnBroadcast")
			h.log(logger.Fatal, err)
		}
	}()
	if len(s.Message) < 4 {
		h.logf(logger.Exploit, "controller broadcast with invalid size")
		return
	}
	switch convert.BytesToUint32(s.Message[:4]) {
	case messages.CMDNodeRegisterResponse:
		h.handleNodeRegisterResponse(s.Message[4:])
	case messages.CMDBeaconRegisterResponse:
		h.handleBeaconRegisterResponse(s.Message[4:])
	case messages.CMDTest:
		h.handleBroadcastTestMessage(s.Message[4:])
	default:
		h.logf(logger.Exploit, "controller broadcast unknown message: %X", s.Message)
	}
}

func (h *handler) handleNodeRegisterResponse(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleNodeRegisterResponse")
			h.log(logger.Fatal, err)
		}
	}()
	nrr := new(messages.NodeRegisterResponse)
	err := msgpack.Unmarshal(message, nrr)
	if err != nil {
		const format = "controller broadcast invalid node register response: %X"
		h.logf(logger.Exploit, format, message)
		return
	}
	h.ctx.storage.AddNodeSessionKey(nrr.GUID, &nodeSessionKey{
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		AckTime:      nrr.ReplyTime,
	})
	h.ctx.storage.SetNodeRegister(nrr.GUID, nrr)
}

func (h *handler) handleBeaconRegisterResponse(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleBeaconRegisterResponse")
			h.log(logger.Fatal, err)
		}
	}()
	brr := new(messages.BeaconRegisterResponse)
	err := msgpack.Unmarshal(message, brr)
	if err != nil {
		const format = "controller broadcast invalid beacon register response: %X"
		h.logf(logger.Exploit, format, message)
		return
	}
	h.ctx.storage.AddBeaconSessionKey(brr.GUID, &beaconSessionKey{
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		AckTime:      brr.ReplyTime,
	})
	h.ctx.storage.SetBeaconRegister(brr.GUID, brr)
}

func (h *handler) handleBroadcastTestMessage(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleBroadcastTestMessage")
			h.log(logger.Fatal, err)
		}
	}()
	if h.ctx.Test.BroadcastTestMsg == nil {
		return
	}
	var testMsg []byte
	err := msgpack.Unmarshal(message, &testMsg)
	if err != nil {
		const format = "controller broadcast invalid test message: %X"
		h.logf(logger.Exploit, format, message)
		return
	}
	select {
	case h.ctx.Test.BroadcastTestMsg <- testMsg:
	case <-h.context.Done():
		return
	}
}

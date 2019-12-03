package controller

import (
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type handler struct {
	ctx *CTRL
}

func (h *handler) logf(l logger.Level, format string, log ...interface{}) {
	h.ctx.logger.Printf(l, "handler", format, log...)
}

func (h *handler) log(l logger.Level, log ...interface{}) {
	h.ctx.logger.Print(l, "handler", log...)
}

// messages from syncer

// TODO maybe need copy data
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

	case messages.Test:
		var testMsg []byte
		err := msgpack.Unmarshal(send.Message[4:], &testMsg)
		if err != nil {
			h.logf(logger.Exploit, "node %X send invalid test message: %X",
				send.RoleGUID, send.Message[4:])
			return
		}
		if h.ctx.Debug.NodeSend != nil {
			h.ctx.Debug.NodeSend <- testMsg
		}
		h.logf(logger.Debug, "node %X send test message: %s",
			send.RoleGUID, string(testMsg))
	default:
		h.logf(logger.Exploit, "node %X send unknown message", send.RoleGUID)
	}
}

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

	case messages.Test:
		var testMsg []byte
		err := msgpack.Unmarshal(send.Message[4:], &testMsg)
		if err != nil {
			h.logf(logger.Exploit, "beacon %X send invalid test message: %X",
				send.RoleGUID, send.Message[4:])
			return
		}
		if h.ctx.Debug.BeaconSend != nil {
			h.ctx.Debug.BeaconSend <- testMsg
		}
		h.logf(logger.Debug, "beacon %X send test message: %s",
			send.RoleGUID, string(testMsg))
	default:
		h.logf(logger.Exploit, "beacon %X send unknown message", send.RoleGUID)
	}
}

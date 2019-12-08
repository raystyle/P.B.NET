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
		if h.ctx.Debug.Send == nil {
			return
		}
		var testMsg []byte
		err := msgpack.Unmarshal(s.Message[4:], &testMsg)
		if err != nil {
			h.logf(logger.Exploit, "controller send invalid test message: %X", s.Message[4:])
			return
		}
		select {
		case h.ctx.Debug.Send <- testMsg:
		case <-h.context.Done():
			return
		}
		h.logf(logger.Debug, "controller send test message: %s", testMsg)
	default:
		h.logf(logger.Exploit, "controller send unknown message: %X", s.Message)
	}
}

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

	case messages.CMDTest:
		if h.ctx.Debug.Broadcast == nil {
			return
		}
		var testMsg []byte
		err := msgpack.Unmarshal(s.Message[4:], &testMsg)
		if err != nil {
			h.logf(logger.Exploit, "controller broadcast invalid test message: %X", s.Message[4:])
			return
		}
		select {
		case h.ctx.Debug.Broadcast <- testMsg:
		case <-h.context.Done():
			return
		}
		h.logf(logger.Debug, "controller broadcast test message: %s", testMsg)
	default:
		h.logf(logger.Exploit, "controller broadcast unknown message: %X", s.Message)
	}
}

func (h *handler) Close() {
	h.cancel()
}

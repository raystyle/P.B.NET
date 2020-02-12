package node

import (
	"bytes"
	"context"
	"fmt"
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
	ctx *Node

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newHandler(ctx *Node) *handler {
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

func (h *handler) logf(lv logger.Level, format string, log ...interface{}) {
	h.ctx.logger.Printf(lv, "handler", format, log...)
}

func (h *handler) log(lv logger.Level, log ...interface{}) {
	h.ctx.logger.Println(lv, "handler", log...)
}

// logfWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo logf
// spew output...
//
// first log interface must be *protocol.Send or *protocol.Broadcast
func (h *handler) logfWithInfo(lv logger.Level, format string, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, format, log[1:]...)
	buf.WriteString("\n")
	spew.Fdump(buf, log[0])
	h.ctx.logger.Print(lv, "handler", buf)
}

// logWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo log
// spew output...
//
// first log interface must be *protocol.Send or *protocol.Broadcast
func (h *handler) logWithInfo(lv logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log[1:]...)
	spew.Fdump(buf, log[0])
	h.ctx.logger.Print(lv, "handler", buf)
}

// logPanic must use like defer h.logPanic("title")
func (h *handler) logPanic(title string) {
	if r := recover(); r != nil {
		h.log(logger.Fatal, xpanic.Print(r, title))
	}
}

// -------------------------------------------send---------------------------------------------------

func (h *handler) OnSend(send *protocol.Send) {
	defer h.logPanic("handler.OnSend")
	if len(send.Message) < 4 {
		const log = "controller send with invalid size"
		h.logWithInfo(logger.Exploit, send, log)
		return
	}
	msgType := convert.BytesToUint32(send.Message[:4])
	send.Message = send.Message[4:]
	switch msgType {
	case messages.CMDAnswerNodeKey:
		h.handleAnswerNodeKey(send)
	case messages.CMDAnswerBeaconKey:
		h.handleAnswerBeaconKey(send)
	case messages.CMDTest:
		h.handleSendTestMessage(send)
	default:
		const format = "controller send unknown message\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, msgType, spew.Sdump(send))
	}
}

func (h *handler) handleAnswerNodeKey(send *protocol.Send) {
	defer h.logPanic("handler.handleAnswerNodeKey")
	ank := new(messages.AnswerNodeKey)
	err := msgpack.Unmarshal(send.Message, ank)
	if err != nil {
		const log = "controller send invalid answer node key data"
		h.logWithInfo(logger.Exploit, send, log)
		return
	}
	err = ank.Validate()
	if err != nil {
		const log = "controller send invalid answer node key"
		h.logWithInfo(logger.Exploit, ank, log)
		return
	}
	h.ctx.storage.AddNodeKey(&ank.GUID, &protocol.NodeKey{
		PublicKey:    ank.PublicKey,
		KexPublicKey: ank.KexPublicKey,
		ReplyTime:    ank.ReplyTime,
	})
}

func (h *handler) handleAnswerBeaconKey(send *protocol.Send) {
	defer h.logPanic("handler.handleAnswerBeaconKey")
	abk := new(messages.AnswerBeaconKey)
	err := msgpack.Unmarshal(send.Message, abk)
	if err != nil {
		const log = "controller send invalid answer beacon key data\nerror:"
		h.logWithInfo(logger.Exploit, send, log, err)
		return
	}
	err = abk.Validate()
	if err != nil {
		const log = "controller send invalid answer beacon key\nerror:"
		h.logWithInfo(logger.Exploit, send, log, err)
		return
	}
	h.ctx.storage.AddBeaconKey(&abk.GUID, &protocol.BeaconKey{
		PublicKey:    abk.PublicKey,
		KexPublicKey: abk.KexPublicKey,
		ReplyTime:    abk.ReplyTime,
	})
}

func (h *handler) handleSendTestMessage(send *protocol.Send) {
	defer h.logPanic("handler.handleSendTestMessage")
	if !h.ctx.Test.testMsgEnabled {
		return
	}
	err := h.ctx.Test.AddSendTestMessage(h.context, send.Message)
	if err != nil {
		const log = "failed to add send test message\nerror:"
		h.logWithInfo(logger.Fatal, send, log, err)
	}
}

// ----------------------------------------broadcast-------------------------------------------------

func (h *handler) OnBroadcast(broadcast *protocol.Broadcast) {
	defer h.logPanic("handler.OnBroadcast")
	if len(broadcast.Message) < 4 {
		const log = "controller broadcast with invalid size"
		h.logWithInfo(logger.Exploit, broadcast, log)
		return
	}
	msgType := convert.BytesToUint32(broadcast.Message[:4])
	broadcast.Message = broadcast.Message[4:]
	switch msgType {
	case messages.CMDNodeRegisterResponse:
		h.handleNodeRegisterResponse(broadcast)
	case messages.CMDBeaconRegisterResponse:
		h.handleBeaconRegisterResponse(broadcast)
	case messages.CMDTest:
		h.handleBroadcastTestMessage(broadcast)
	default:
		const format = "controller broadcast unknown message\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, msgType, spew.Sdump(broadcast))
	}
}

func (h *handler) handleNodeRegisterResponse(broadcast *protocol.Broadcast) {
	defer h.logPanic("handler.handleNodeRegisterResponse")
	nrr := new(messages.NodeRegisterResponse)
	err := msgpack.Unmarshal(broadcast.Message, nrr)
	if err != nil {
		const log = "controller broadcast invalid node register response data\nerror:"
		h.logWithInfo(logger.Exploit, broadcast, log, err)
		return
	}
	err = nrr.Validate()
	if err != nil {
		const log = "controller broadcast invalid node register response\nerror:"
		h.logWithInfo(logger.Exploit, nrr, log, err)
		return
	}
	h.ctx.storage.AddNodeKey(&nrr.GUID, &protocol.NodeKey{
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		ReplyTime:    nrr.ReplyTime,
	})
	h.ctx.storage.SetNodeRegister(&nrr.GUID, nrr)
}

func (h *handler) handleBeaconRegisterResponse(broadcast *protocol.Broadcast) {
	defer h.logPanic("handler.handleBeaconRegisterResponse")
	brr := new(messages.BeaconRegisterResponse)
	err := msgpack.Unmarshal(broadcast.Message, brr)
	if err != nil {
		const log = "controller broadcast invalid beacon register response data"
		h.logWithInfo(logger.Exploit, broadcast, log)
		return
	}
	err = brr.Validate()
	if err != nil {
		const log = "controller broadcast invalid beacon register response"
		h.logWithInfo(logger.Exploit, brr, log)
		return
	}
	h.ctx.storage.AddBeaconKey(&brr.GUID, &protocol.BeaconKey{
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		ReplyTime:    brr.ReplyTime,
	})
	h.ctx.storage.SetBeaconRegister(&brr.GUID, brr)
}

func (h *handler) handleBroadcastTestMessage(broadcast *protocol.Broadcast) {
	defer h.logPanic("handler.handleBroadcastTestMessage")
	if !h.ctx.Test.testMsgEnabled {
		return
	}
	err := h.ctx.Test.AddBroadcastTestMessage(h.context, broadcast.Message)
	if err != nil {
		const log = "failed to add broadcast test message\nerror:"
		h.logWithInfo(logger.Fatal, broadcast, log, err)
	}
}

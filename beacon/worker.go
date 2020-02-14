package beacon

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"hash"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type worker struct {
	sendQueue   chan *protocol.Send
	ackQueue    chan *protocol.Acknowledge
	answerQueue chan *protocol.Answer

	sendPool   sync.Pool
	ackPool    sync.Pool
	answerPool sync.Pool

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newWorker(ctx *Beacon, config *Config) (*worker, error) {
	cfg := config.Worker

	if cfg.Number < 4 {
		return nil, errors.New("worker number must >= 4")
	}
	if cfg.QueueSize < cfg.Number {
		return nil, errors.New("worker task queue size < worker number")
	}
	if cfg.MaxBufferSize < 16<<10 {
		return nil, errors.New("worker max buffer size must >= 16KB")
	}

	worker := worker{
		sendQueue:   make(chan *protocol.Send, cfg.QueueSize),
		ackQueue:    make(chan *protocol.Acknowledge, cfg.QueueSize),
		answerQueue: make(chan *protocol.Answer, cfg.QueueSize),
		stopSignal:  make(chan struct{}),
	}

	worker.sendPool.New = func() interface{} {
		return protocol.NewSend()
	}
	worker.ackPool.New = func() interface{} {
		return protocol.NewAcknowledge()
	}
	worker.answerPool.New = func() interface{} {
		return protocol.NewAnswer()
	}

	// start sub workers
	sendPoolP := &worker.sendPool
	ackPoolP := &worker.ackPool
	answerPoolP := &worker.answerPool
	wgP := &worker.wg
	worker.wg.Add(2 * cfg.Number)
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx:           ctx,
			maxBufferSize: cfg.MaxBufferSize,
			sendQueue:     worker.sendQueue,
			ackQueue:      worker.ackQueue,
			answerQueue:   worker.answerQueue,
			sendPool:      sendPoolP,
			ackPool:       ackPoolP,
			answerPool:    answerPoolP,
			stopSignal:    worker.stopSignal,
			wg:            wgP,
		}
		go sw.WorkWithBlock()
	}
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx:           ctx,
			maxBufferSize: cfg.MaxBufferSize,
			ackQueue:      worker.ackQueue,
			ackPool:       ackPoolP,
			stopSignal:    worker.stopSignal,
			wg:            wgP,
		}
		go sw.WorkWithNonBlock()
	}
	return &worker, nil
}

// GetSendFromPool is used to get *protocol.Send from sendPool
func (ws *worker) GetSendFromPool() *protocol.Send {
	return ws.sendPool.Get().(*protocol.Send)
}

// PutSendToPool is used to put *protocol.Send to sendPool
func (ws *worker) PutSendToPool(s *protocol.Send) {
	ws.sendPool.Put(s)
}

// GetAcknowledgeFromPool is used to get *protocol.Acknowledge from ackPool
func (ws *worker) GetAcknowledgeFromPool() *protocol.Acknowledge {
	return ws.ackPool.Get().(*protocol.Acknowledge)
}

// PutAcknowledgeToPool is used to put *protocol.Acknowledge to ackPool
func (ws *worker) PutAcknowledgeToPool(a *protocol.Acknowledge) {
	ws.ackPool.Put(a)
}

// GetAnswerFromPool is used to get *protocol.Answer from answerPool
func (ws *worker) GetAnswerFromPool() *protocol.Answer {
	return ws.answerPool.Get().(*protocol.Answer)
}

// PutAnswerToPool is used to put *protocol.Answer to answerPool
func (ws *worker) PutAnswerToPool(a *protocol.Answer) {
	ws.answerPool.Put(a)
}

// AddSend is used to add send to sub workers
func (ws *worker) AddSend(s *protocol.Send) {
	select {
	case ws.sendQueue <- s:
	case <-ws.stopSignal:
	}
}

// AddAcknowledge is used to add acknowledge to sub workers
func (ws *worker) AddAcknowledge(a *protocol.Acknowledge) {
	select {
	case ws.ackQueue <- a:
	case <-ws.stopSignal:
	}
}

// AddAnswer is used to add answer to sub workers
func (ws *worker) AddAnswer(a *protocol.Answer) {
	select {
	case ws.answerQueue <- a:
	case <-ws.stopSignal:
	}
}

// Close is used to close all sub workers
func (ws *worker) Close() {
	close(ws.stopSignal)
	ws.wg.Wait()
}

type subWorker struct {
	ctx *Beacon

	maxBufferSize int

	sendQueue   chan *protocol.Send
	ackQueue    chan *protocol.Acknowledge
	answerQueue chan *protocol.Answer

	sendPool   *sync.Pool
	ackPool    *sync.Pool
	answerPool *sync.Pool

	// runtime
	buffer *bytes.Buffer
	hash   hash.Hash
	timer  *time.Timer
	err    error

	stopSignal chan struct{}
	wg         *sync.WaitGroup
}

func (sw *subWorker) logf(lv logger.Level, format string, log ...interface{}) {
	sw.ctx.logger.Printf(lv, "worker", format, log...)
}

func (sw *subWorker) log(lv logger.Level, log ...interface{}) {
	sw.ctx.logger.Println(lv, "worker", log...)
}

func (sw *subWorker) WorkWithBlock() {
	defer func() {
		if r := recover(); r != nil {
			sw.log(logger.Fatal, xpanic.Print(r, "subWorker.WorkWithBlock"))
			// restart worker
			time.Sleep(time.Second)
			go sw.WorkWithBlock()
		} else {
			sw.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.hash = sha256.New()
	sw.timer = time.NewTimer(time.Second)
	sw.timer.Stop()
	defer sw.timer.Stop()
	var (
		send        *protocol.Send
		acknowledge *protocol.Acknowledge
		answer      *protocol.Answer
	)
	for {
		select {
		case <-sw.stopSignal:
			return
		default:
		}
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
		}
		select {
		case send = <-sw.sendQueue:
			sw.handleSend(send)
		case acknowledge = <-sw.ackQueue:
			sw.handleAcknowledge(acknowledge)
		case answer = <-sw.answerQueue:
			sw.handleAnswer(answer)
		case <-sw.stopSignal:
			return
		}
	}
}

func (sw *subWorker) WorkWithNonBlock() {
	defer func() {
		if r := recover(); r != nil {
			sw.log(logger.Fatal, xpanic.Print(r, "subWorker.WorkWithNonBlock"))
			// restart worker
			time.Sleep(time.Second)
			go sw.WorkWithNonBlock()
		} else {
			sw.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.timer = time.NewTimer(time.Second)
	defer sw.timer.Stop()
	var acknowledge *protocol.Acknowledge
	for {
		select {
		case <-sw.stopSignal:
			return
		default:
		}
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
		}
		select {
		case acknowledge = <-sw.ackQueue:
			sw.handleAcknowledge(acknowledge)
		case <-sw.stopSignal:
			return
		}
	}
}

func (sw *subWorker) handleSend(send *protocol.Send) {
	defer sw.sendPool.Put(send)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(send.GUID[:])
	sw.buffer.Write(send.RoleGUID[:])
	sw.buffer.Write(send.Hash)
	sw.buffer.Write(send.Message)
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), send.Signature) {
		const format = "invalid send signature\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(send))
		return
	}
	// decrypt message
	send.Message, sw.err = sw.ctx.global.Decrypt(send.Message)
	if sw.err != nil {
		const format = "failed to decrypt send message: %s\n%s"
		sw.logf(logger.Exploit, format, sw.err, spew.Sdump(send))
		return
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(send.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), send.Hash) != 1 {
		const format = "send with incorrect hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(send))
		return
	}
	// create answer for OnMessage
	answer := sw.answerPool.Get().(*protocol.Answer)
	defer sw.answerPool.Put(answer)
	answer.GUID = send.GUID
	answer.BeaconGUID = send.RoleGUID
	// must use copy, because use two sync.Pool
	copy(answer.Hash, send.Hash)
	copy(answer.Signature, send.Signature)
	// copy send.Message to answer.Message
	smLen := len(send.Message)
	amLen := len(answer.Message)
	if cap(answer.Message) >= smLen {
		switch {
		case amLen > smLen:
			copy(answer.Message, send.Message)
			answer.Message = answer.Message[:smLen]
		case amLen == smLen:
			copy(answer.Message, send.Message)
		case amLen < smLen:
			answer.Message = append(answer.Message[:0], send.Message...)
		}
	} else {
		answer.Message = make([]byte, smLen)
		copy(answer.Message, send.Message)
	}
	sw.ctx.handler.OnMessage(answer)
	for {
		sw.err = sw.ctx.sender.Acknowledge(send)
		if sw.err == nil {
			return
		}
		if sw.err == ErrNoConnections {
			sw.log(logger.Warning, "failed to acknowledge:", sw.err)
		} else {
			sw.log(logger.Error, "failed to acknowledge:", sw.err)
		}
		// wait one second
		sw.timer.Reset(time.Second)
		select {
		case <-sw.timer.C:
		case <-sw.stopSignal:
			return
		}
	}
}

func (sw *subWorker) handleAcknowledge(acknowledge *protocol.Acknowledge) {
	defer sw.ackPool.Put(acknowledge)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(acknowledge.GUID[:])
	sw.buffer.Write(acknowledge.RoleGUID[:])
	sw.buffer.Write(acknowledge.SendGUID[:])
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), acknowledge.Signature) {
		const format = "invalid acknowledge signature\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(acknowledge))
		return
	}
	sw.ctx.sender.HandleAcknowledge(&acknowledge.SendGUID)
}

func (sw *subWorker) handleAnswer(answer *protocol.Answer) {
	defer sw.answerPool.Put(answer)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(answer.GUID[:])
	sw.buffer.Write(answer.BeaconGUID[:])
	sw.buffer.Write(convert.Uint64ToBytes(answer.Index))
	sw.buffer.Write(answer.Hash)
	sw.buffer.Write(answer.Message)
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), answer.Signature) {
		const format = "invalid answer signature\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(answer))
		return
	}
	// decrypt message
	answer.Message, sw.err = sw.ctx.global.Decrypt(answer.Message)
	if sw.err != nil {
		const format = "failed to decrypt answer message: %s\n%s"
		sw.logf(logger.Exploit, format, sw.err, spew.Sdump(answer))
		return
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(answer.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), answer.Hash) != 1 {
		const format = "answer with incorrect hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(answer))
		return
	}
	// prevent duplicate handle
	if !sw.ctx.sender.CheckQueryIndex(answer.Index) {
		return
	}
	sw.ctx.handler.OnMessage(answer)
}

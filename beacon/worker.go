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
	worker.wg.Add(cfg.Number)
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
		go sw.Work()
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
	err    error

	stopSignal chan struct{}
	wg         *sync.WaitGroup
}

func (sw *subWorker) logf(l logger.Level, format string, log ...interface{}) {
	sw.ctx.logger.Printf(l, "worker", format, log...)
}

func (sw *subWorker) log(l logger.Level, log ...interface{}) {
	sw.ctx.logger.Println(l, "worker", log...)
}

func (sw *subWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			sw.log(logger.Fatal, xpanic.Print(r, "subWorker.Work()"))
			// restart worker
			time.Sleep(time.Second)
			go sw.Work()
		} else {
			sw.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.hash = sha256.New()
	var (
		send   *protocol.Send
		ack    *protocol.Acknowledge
		answer *protocol.Answer
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
		case ack = <-sw.ackQueue:
			sw.handleAcknowledge(ack)
		case answer = <-sw.answerQueue:
			sw.handleAnswer(answer)
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
	cache := send.Message
	defer func() { send.Message = cache }()
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
	sw.ctx.sender.Acknowledge(send)
	sw.ctx.handler.OnMessage(send)
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
	// sw.buffer.Reset()
	// sw.buffer.Write(broadcast.GUID[:])
	// sw.buffer.Write(broadcast.Hash)
	// sw.buffer.Write(broadcast.Message)
	// if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), broadcast.Signature) {
	// 	const format = "invalid broadcast signature\n%s"
	// 	sw.logf(logger.Exploit, format, spew.Sdump(broadcast))
	// 	return
	// }
	// // decrypt message
	// cache := broadcast.Message
	// defer func() { broadcast.Message = cache }()
	// broadcast.Message, sw.err = sw.ctx.global.CtrlDecrypt(broadcast.Message)
	// if sw.err != nil {
	// 	const format = "failed to decrypt broadcast message: %s\n%s"
	// 	sw.logf(logger.Exploit, format, sw.err, spew.Sdump(broadcast))
	// 	return
	// }
	// // compare hash
	// sw.hash.Reset()
	// sw.hash.Write(broadcast.Message)
	// if subtle.ConstantTimeCompare(sw.hash.Sum(nil), broadcast.Hash) != 1 {
	// 	const format = "broadcast with incorrect hash\n%s"
	// 	sw.logf(logger.Exploit, format, spew.Sdump(broadcast))
	// 	return
	// }
	// sw.ctx.handler.OnBroadcast(broadcast)
}

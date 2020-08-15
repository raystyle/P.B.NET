package beacon

import (
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"crypto/subtle"
	"hash"
	"io"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/crypto/hmac"
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
	hmacPool   sync.Pool

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
	sessionKey := ctx.global.SessionKey()
	worker.hmacPool.New = func() interface{} {
		key := sessionKey.Get()
		defer sessionKey.Put(key)
		return hmac.New(sha256.New, key)
	}

	// start sub workers
	sendPoolP := &worker.sendPool
	ackPoolP := &worker.ackPool
	answerPoolP := &worker.answerPool
	hmacPoolP := &worker.hmacPool
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
			hmacPool:      hmacPoolP,
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
			hmacPool:      hmacPoolP,
			stopSignal:    worker.stopSignal,
			wg:            wgP,
		}
		go sw.WorkWithoutBlock()
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

	// copy from worker
	sendQueue   chan *protocol.Send
	ackQueue    chan *protocol.Acknowledge
	answerQueue chan *protocol.Answer

	sendPool   *sync.Pool
	ackPool    *sync.Pool
	answerPool *sync.Pool
	hmacPool   *sync.Pool

	// runtime
	buffer  *bytes.Buffer
	reader  *bytes.Reader
	deflate io.ReadCloser
	hash    hash.Hash
	timer   *time.Timer
	err     error

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
	sw.reader = bytes.NewReader(nil)
	sw.deflate = flate.NewReader(nil)
	sw.hash = sha256.New()
	// must stop at once, or maybe timeout at the first time.
	sw.timer = time.NewTimer(time.Minute)
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

func (sw *subWorker) WorkWithoutBlock() {
	defer func() {
		if r := recover(); r != nil {
			sw.log(logger.Fatal, xpanic.Print(r, "subWorker.WorkWithoutBlock"))
			// restart worker
			time.Sleep(time.Second)
			go sw.WorkWithoutBlock()
		} else {
			sw.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.reader = bytes.NewReader(nil)
	sw.deflate = flate.NewReader(nil)
	sw.hash = sha256.New()
	var (
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
		case acknowledge = <-sw.ackQueue:
			sw.handleAcknowledge(acknowledge)
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
	if subtle.ConstantTimeCompare(sw.calculateSendHMAC(send), send.Hash) != 1 {
		const format = "send with incorrect hmac hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(send))
		return
	}
	// decrypt compressed message
	send.Message, sw.err = sw.ctx.global.Decrypt(send.Message)
	if sw.err != nil {
		const format = "failed to decrypt send message: %s\n%s"
		sw.logf(logger.Exploit, format, sw.err, spew.Sdump(send))
		return
	}
	// decompress message
	if send.Deflate == 1 {
		sw.reader.Reset(send.Message)
		sw.err = sw.deflate.(flate.Resetter).Reset(sw.reader, nil)
		if sw.err != nil {
			const format = "failed to reset deflate reader about send message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(send))
			return
		}
		sw.buffer.Reset()
		_, sw.err = sw.buffer.ReadFrom(sw.deflate)
		if sw.err != nil {
			const format = "failed to decompress send message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(send))
			return
		}
		sw.err = sw.deflate.Close()
		if sw.err != nil {
			const format = "failed to close deflate reader about send message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(send))
			return
		}
		// must recover it, otherwise will appear data race
		aesBuffer := send.Message
		defer func() { send.Message = aesBuffer }()
		send.Message = sw.buffer.Bytes()
	}
	// create answer for OnMessage
	answer := sw.answerPool.Get().(*protocol.Answer)
	defer sw.answerPool.Put(answer)
	sw.copySendToAnswer(answer, send)
	sw.ctx.handler.OnMessage(answer)
	for {
		sw.err = sw.ctx.sender.Acknowledge(send)
		if sw.err == nil {
			return
		}
		if sw.err == ErrNoConnections || sw.err == ErrFailedToAck {
			sw.log(logger.Warning, "failed to acknowledge:", sw.err)
		} else {
			sw.log(logger.Error, "failed to acknowledge:", sw.err)
			return
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

func (sw *subWorker) calculateSendHMAC(send *protocol.Send) []byte {
	h := sw.hmacPool.Get().(hash.Hash)
	defer sw.hmacPool.Put(h)
	h.Reset()
	h.Write(send.GUID[:])
	h.Write(send.RoleGUID[:])
	h.Write([]byte{send.Deflate})
	h.Write(send.Message)
	return h.Sum(nil)
}

func (sw *subWorker) copySendToAnswer(answer *protocol.Answer, send *protocol.Send) {
	answer.GUID = send.GUID
	answer.BeaconGUID = send.RoleGUID
	answer.Deflate = send.Deflate
	// must use copy, because use two sync.Pool
	copy(answer.Hash, send.Hash)
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
}

func (sw *subWorker) handleAcknowledge(ack *protocol.Acknowledge) {
	defer sw.ackPool.Put(ack)
	// verify
	if subtle.ConstantTimeCompare(sw.calculateAcknowledgeHMAC(ack), ack.Hash) != 1 {
		const format = "acknowledge with incorrect hmac hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(ack))
		return
	}
	sw.ctx.sender.HandleAcknowledge(&ack.SendGUID)
}

func (sw *subWorker) calculateAcknowledgeHMAC(ack *protocol.Acknowledge) []byte {
	h := sw.hmacPool.Get().(hash.Hash)
	defer sw.hmacPool.Put(h)
	h.Reset()
	h.Write(ack.GUID[:])
	h.Write(ack.RoleGUID[:])
	h.Write(ack.SendGUID[:])
	return h.Sum(nil)
}

func (sw *subWorker) handleAnswer(answer *protocol.Answer) {
	defer sw.answerPool.Put(answer)
	// verify
	if subtle.ConstantTimeCompare(sw.calculateAnswerHMAC(answer), answer.Hash) != 1 {
		const format = "answer with incorrect hmac hash\n%s"
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
	// decompress message
	if answer.Deflate == 1 {
		sw.reader.Reset(answer.Message)
		sw.err = sw.deflate.(flate.Resetter).Reset(sw.reader, nil)
		if sw.err != nil {
			const format = "failed to reset deflate reader about answer message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(answer))
			return
		}
		sw.buffer.Reset()
		_, sw.err = sw.buffer.ReadFrom(sw.deflate)
		if sw.err != nil {
			const format = "failed to decompress answer message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(answer))
			return
		}
		sw.err = sw.deflate.Close()
		if sw.err != nil {
			const format = "failed to close deflate reader about answer message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(answer))
			return
		}
		// must recover it, otherwise will appear data race.
		aesBuffer := answer.Message
		defer func() { answer.Message = aesBuffer }()
		answer.Message = sw.buffer.Bytes()
	}
	// prevent duplicate handle
	if !sw.ctx.sender.AddQueryIndex(answer.Index) {
		return
	}
	sw.ctx.handler.OnMessage(answer)
}

func (sw *subWorker) calculateAnswerHMAC(answer *protocol.Answer) []byte {
	h := sw.hmacPool.Get().(hash.Hash)
	defer sw.hmacPool.Put(h)
	h.Reset()
	h.Write(answer.GUID[:])
	h.Write(answer.BeaconGUID[:])
	h.Write(convert.BEUint64ToBytes(answer.Index))
	h.Write([]byte{answer.Deflate})
	h.Write(answer.Message)
	return h.Sum(nil)
}

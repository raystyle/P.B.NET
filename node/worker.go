package node

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

	"project/internal/crypto/hmac"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

// worker is used to handle message from controller.
type worker struct {
	sendQueue        chan *protocol.Send
	acknowledgeQueue chan *protocol.Acknowledge
	broadcastQueue   chan *protocol.Broadcast

	sendPool        sync.Pool
	acknowledgePool sync.Pool
	broadcastPool   sync.Pool
	hmacPool        sync.Pool

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newWorker(ctx *Node, config *Config) (*worker, error) {
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
		sendQueue:        make(chan *protocol.Send, cfg.QueueSize),
		acknowledgeQueue: make(chan *protocol.Acknowledge, cfg.QueueSize),
		broadcastQueue:   make(chan *protocol.Broadcast, cfg.QueueSize),
		stopSignal:       make(chan struct{}),
	}

	worker.sendPool.New = func() interface{} {
		return protocol.NewSend()
	}
	worker.acknowledgePool.New = func() interface{} {
		return protocol.NewAcknowledge()
	}
	worker.broadcastPool.New = func() interface{} {
		return protocol.NewBroadcast()
	}
	sessionKey := ctx.global.SessionKey()
	worker.hmacPool.New = func() interface{} {
		key := sessionKey.Get()
		defer sessionKey.Put(key)
		return hmac.New(sha256.New, key)
	}

	// start sub workers
	sendPoolP := &worker.sendPool
	acknowledgePoolP := &worker.acknowledgePool
	broadcastPoolP := &worker.broadcastPool
	hmacPoolP := &worker.hmacPool
	wgP := &worker.wg
	worker.wg.Add(2 * cfg.Number)
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx:              ctx,
			maxBufferSize:    cfg.MaxBufferSize,
			sendQueue:        worker.sendQueue,
			acknowledgeQueue: worker.acknowledgeQueue,
			broadcastQueue:   worker.broadcastQueue,
			sendPool:         sendPoolP,
			acknowledgePool:  acknowledgePoolP,
			broadcastPool:    broadcastPoolP,
			hmacPool:         hmacPoolP,
			stopSignal:       worker.stopSignal,
			wg:               wgP,
		}
		go sw.WorkWithBlock()
	}
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx:              ctx,
			maxBufferSize:    cfg.MaxBufferSize,
			acknowledgeQueue: worker.acknowledgeQueue,
			acknowledgePool:  acknowledgePoolP,
			hmacPool:         hmacPoolP,
			stopSignal:       worker.stopSignal,
			wg:               wgP,
		}
		go sw.WorkWithoutBlock()
	}
	return &worker, nil
}

// GetSendFromPool is used to get *protocol.Send from sendPool
func (worker *worker) GetSendFromPool() *protocol.Send {
	return worker.sendPool.Get().(*protocol.Send)
}

// PutSendToPool is used to put *protocol.Send to sendPool
func (worker *worker) PutSendToPool(s *protocol.Send) {
	worker.sendPool.Put(s)
}

// GetAcknowledgeFromPool is used to get *protocol.Acknowledge from acknowledgePool
func (worker *worker) GetAcknowledgeFromPool() *protocol.Acknowledge {
	return worker.acknowledgePool.Get().(*protocol.Acknowledge)
}

// PutAcknowledgeToPool is used to put *protocol.Acknowledge to acknowledgePool
func (worker *worker) PutAcknowledgeToPool(a *protocol.Acknowledge) {
	worker.acknowledgePool.Put(a)
}

// GetBroadcastFromPool is used to get *protocol.Broadcast from broadcastPool
func (worker *worker) GetBroadcastFromPool() *protocol.Broadcast {
	return worker.broadcastPool.Get().(*protocol.Broadcast)
}

// PutBroadcastToPool is used to put *protocol.Broadcast to broadcastPool
func (worker *worker) PutBroadcastToPool(b *protocol.Broadcast) {
	worker.broadcastPool.Put(b)
}

// AddSend is used to add send to sub workers
func (worker *worker) AddSend(s *protocol.Send) {
	select {
	case worker.sendQueue <- s:
	case <-worker.stopSignal:
	}
}

// AddAcknowledge is used to add acknowledge to sub workers
func (worker *worker) AddAcknowledge(a *protocol.Acknowledge) {
	select {
	case worker.acknowledgeQueue <- a:
	case <-worker.stopSignal:
	}
}

// AddBroadcast is used to add broadcast to sub workers
func (worker *worker) AddBroadcast(b *protocol.Broadcast) {
	select {
	case worker.broadcastQueue <- b:
	case <-worker.stopSignal:
	}
}

// Close is used to close all sub workers
func (worker *worker) Close() {
	close(worker.stopSignal)
	worker.wg.Wait()
}

type subWorker struct {
	ctx *Node

	maxBufferSize int

	// copy from worker
	sendQueue        chan *protocol.Send
	acknowledgeQueue chan *protocol.Acknowledge
	broadcastQueue   chan *protocol.Broadcast

	sendPool        *sync.Pool
	acknowledgePool *sync.Pool
	broadcastPool   *sync.Pool
	hmacPool        *sync.Pool

	// runtime
	buffer  *bytes.Buffer
	reader  *bytes.Reader
	deflate io.ReadCloser
	hash    hash.Hash // for broadcast
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
		broadcast   *protocol.Broadcast
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
		case acknowledge = <-sw.acknowledgeQueue:
			sw.handleAcknowledge(acknowledge)
		case broadcast = <-sw.broadcastQueue:
			sw.handleBroadcast(broadcast)
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
		broadcast   *protocol.Broadcast
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
		case acknowledge = <-sw.acknowledgeQueue:
			sw.handleAcknowledge(acknowledge)
		case broadcast = <-sw.broadcastQueue:
			sw.handleBroadcast(broadcast)
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
	// decrypt message
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
	sw.ctx.handler.OnSend(send)
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

func (sw *subWorker) handleAcknowledge(ack *protocol.Acknowledge) {
	defer sw.acknowledgePool.Put(ack)
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

func (sw *subWorker) handleBroadcast(broadcast *protocol.Broadcast) {
	defer sw.broadcastPool.Put(broadcast)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(broadcast.GUID[:])
	sw.buffer.Write(broadcast.Hash)
	sw.buffer.WriteByte(broadcast.Deflate)
	sw.buffer.Write(broadcast.Message)
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), broadcast.Signature) {
		const format = "invalid broadcast signature\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(broadcast))
		return
	}
	// decrypt message
	broadcast.Message, sw.err = sw.ctx.global.CtrlDecrypt(broadcast.Message)
	if sw.err != nil {
		const format = "failed to decrypt broadcast message: %s\n%s"
		sw.logf(logger.Exploit, format, sw.err, spew.Sdump(broadcast))
		return
	}
	// decompress message
	if broadcast.Deflate == 1 {
		sw.reader.Reset(broadcast.Message)
		sw.err = sw.deflate.(flate.Resetter).Reset(sw.reader, nil)
		if sw.err != nil {
			const format = "failed to reset deflate reader from broadcast message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(broadcast))
			return
		}
		sw.buffer.Reset()
		_, sw.err = sw.buffer.ReadFrom(sw.deflate)
		if sw.err != nil {
			const format = "failed to decompress broadcast message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(broadcast))
			return
		}
		sw.err = sw.deflate.Close()
		if sw.err != nil {
			const format = "failed to close deflate reader about broadcast message: %s\n%s"
			sw.logf(logger.Exploit, format, sw.err, spew.Sdump(broadcast))
			return
		}
		// must recover it, otherwise will appear data race
		aesBuffer := broadcast.Message
		defer func() { broadcast.Message = aesBuffer }()
		broadcast.Message = sw.buffer.Bytes()
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(broadcast.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), broadcast.Hash) != 1 {
		const format = "broadcast with incorrect hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(broadcast))
		return
	}
	sw.ctx.handler.OnBroadcast(broadcast)
}

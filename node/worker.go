package node

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

// worker is used to handle message from controller
type worker struct {
	sendQueue        chan *protocol.Send
	acknowledgeQueue chan *protocol.Acknowledge
	broadcastQueue   chan *protocol.Broadcast

	sendPool        sync.Pool
	acknowledgePool sync.Pool
	broadcastPool   sync.Pool

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

	// start sub workers
	sendPoolP := &worker.sendPool
	acknowledgePoolP := &worker.acknowledgePool
	broadcastPoolP := &worker.broadcastPool
	wgP := &worker.wg
	worker.wg.Add(cfg.Number)
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
			stopSignal:       worker.stopSignal,
			wg:               wgP,
		}
		go sw.Work()
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

	sendQueue        chan *protocol.Send
	acknowledgeQueue chan *protocol.Acknowledge
	broadcastQueue   chan *protocol.Broadcast

	sendPool        *sync.Pool
	acknowledgePool *sync.Pool
	broadcastPool   *sync.Pool

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
	sw.ctx.handler.OnSend(send)
	sw.ctx.sender.Acknowledge(send)
}

func (sw *subWorker) handleAcknowledge(acknowledge *protocol.Acknowledge) {
	defer sw.acknowledgePool.Put(acknowledge)
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

func (sw *subWorker) handleBroadcast(broadcast *protocol.Broadcast) {
	defer sw.broadcastPool.Put(broadcast)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(broadcast.GUID[:])
	sw.buffer.Write(broadcast.Hash)
	sw.buffer.Write(broadcast.Message)
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), broadcast.Signature) {
		const format = "invalid broadcast signature\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(broadcast))
		return
	}
	// decrypt message
	cache := broadcast.Message
	defer func() { broadcast.Message = cache }()
	broadcast.Message, sw.err = sw.ctx.global.CtrlDecrypt(broadcast.Message)
	if sw.err != nil {
		const format = "failed to decrypt broadcast message: %s\n%s"
		sw.logf(logger.Exploit, format, sw.err, spew.Sdump(broadcast))
		return
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

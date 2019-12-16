package node

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"hash"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

// worker is used to handle message from controller
type worker struct {
	broadcastQueue chan *protocol.Broadcast
	sendQueue      chan *protocol.Send

	broadcastPool sync.Pool
	sendPool      sync.Pool

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
	if cfg.MaxBufferSize < 16384 {
		return nil, errors.New("max buffer size must >= 16384")
	}

	worker := worker{
		broadcastQueue: make(chan *protocol.Broadcast, cfg.QueueSize),
		sendQueue:      make(chan *protocol.Send, cfg.QueueSize),
		stopSignal:     make(chan struct{}),
	}

	worker.broadcastPool.New = func() interface{} {
		return new(protocol.Broadcast)
	}

	worker.sendPool.New = func() interface{} {
		return new(protocol.Send)
	}

	// start sub workers
	broadcastPoolP := &worker.broadcastPool
	sendPoolP := &worker.sendPool
	wgP := &worker.wg
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx: ctx,

			maxBufferSize: cfg.MaxBufferSize,

			broadcastQueue: worker.broadcastQueue,
			sendQueue:      worker.sendQueue,
			broadcastPool:  broadcastPoolP,
			sendPool:       sendPoolP,

			stopSignal: worker.stopSignal,
			wg:         wgP,
		}
		worker.wg.Add(1)
		go sw.Work()
	}
	return &worker, nil
}

// GetBroadcastFromPool is used to get *protocol.Broadcast from broadcastPool
func (ws *worker) GetBroadcastFromPool() *protocol.Broadcast {
	return ws.broadcastPool.Get().(*protocol.Broadcast)
}

// GetSendFromPool is used to get *protocol.Send from sendPool
func (ws *worker) GetSendFromPool() *protocol.Send {
	return ws.sendPool.Get().(*protocol.Send)
}

// AddBroadcast is used to add broadcast to sub workers
func (ws *worker) AddBroadcast(b *protocol.Broadcast) {
	select {
	case ws.broadcastQueue <- b:
	case <-ws.stopSignal:
	}
}

// AddNodeSend is used to add send to sub workers
func (ws *worker) AddSend(s *protocol.Send) {
	select {
	case ws.sendQueue <- s:
	case <-ws.stopSignal:
	}
}

// Close is used to close all sub workers
func (ws *worker) Close() {
	close(ws.stopSignal)
	ws.wg.Wait()
}

type subWorker struct {
	ctx *Node

	maxBufferSize int

	broadcastQueue chan *protocol.Broadcast
	sendQueue      chan *protocol.Send
	broadcastPool  *sync.Pool
	sendPool       *sync.Pool

	// runtime
	buffer  *bytes.Buffer
	hash    hash.Hash
	guid    *guid.Generator
	ack     *protocol.Acknowledge
	encoder *msgpack.Encoder
	err     error

	stopSignal chan struct{}
	wg         *sync.WaitGroup
}

func (sw *subWorker) logf(l logger.Level, format string, log ...interface{}) {
	sw.ctx.logger.Printf(l, "worker", format, log...)
}

func (sw *subWorker) log(l logger.Level, log ...interface{}) {
	sw.ctx.logger.Print(l, "worker", log...)
}

func (sw *subWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			sw.log(logger.Fatal, xpanic.Error(r, "subWorker.Work()"))
			// restart worker
			time.Sleep(time.Second)
			go sw.Work()
		} else {
			sw.guid.Close()
			sw.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.hash = sha256.New()
	sw.guid = guid.New(len(sw.sendQueue), sw.ctx.global.Now)
	sw.ack = &protocol.Acknowledge{SendGUID: make([]byte, guid.Size)}
	sw.encoder = msgpack.NewEncoder(sw.buffer)
	var (
		b *protocol.Broadcast
		s *protocol.Send
	)
	for {
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
		}
		select {
		case <-sw.stopSignal:
			return
		default:
		}
		select {
		case b = <-sw.broadcastQueue:
			sw.handleBroadcast(b)
		case s = <-sw.sendQueue:
			sw.handleSend(s)
		case <-sw.stopSignal:
			return
		}
	}
}

func (sw *subWorker) handleBroadcast(b *protocol.Broadcast) {
	defer sw.broadcastPool.Put(b)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(b.GUID)
	sw.buffer.Write(b.Message)
	sw.buffer.Write(b.Hash)
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), b.Signature) {
		const format = "invalid broadcast signature\nGUID: %X"
		sw.logf(logger.Exploit, format, b.GUID)
		return
	}
	// decrypt message
	b.Message, sw.err = sw.ctx.global.CtrlDecrypt(b.Message)
	if sw.err != nil {
		const format = "failed to decrypt broadcast message: %s\nGUID: %X"
		sw.logf(logger.Exploit, format, sw.err, b.GUID)
		return
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(b.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), b.Hash) != 1 {
		const format = "broadcast with incorrect hash\nGUID: %X"
		sw.logf(logger.Exploit, format, b.GUID)
		return
	}
	// handle it, don't need acknowledge
	sw.ctx.handler.OnBroadcast(b)
}

func (sw *subWorker) handleSend(s *protocol.Send) {
	defer sw.sendPool.Put(s)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(s.GUID)
	sw.buffer.Write(s.RoleGUID)
	sw.buffer.Write(s.Message)
	sw.buffer.Write(s.Hash)
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), s.Signature) {
		const format = "invalid send signature\nGUID: %X"
		sw.logf(logger.Exploit, format, s.GUID)
		return
	}
	// decrypt message
	s.Message, sw.err = sw.ctx.global.Decrypt(s.Message)
	if sw.err != nil {
		const format = "failed to decrypt send message: %s\nGUID: %X"
		sw.logf(logger.Exploit, format, sw.err, s.GUID)
		return
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(s.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), s.Hash) != 1 {
		const format = "send with incorrect hash\nGUID: %X"
		sw.logf(logger.Exploit, format, s.GUID)
		return
	}
	sw.acknowledge(s)
	sw.ctx.handler.OnSend(s)
}

func (sw *subWorker) acknowledge(s *protocol.Send) {
	sw.ack.GUID = sw.guid.Get()
	sw.ack.RoleGUID = sw.ctx.global.GUID()
	// must copy because s is from sync pool and
	// sw.ctx.forwarder.AckToNodeAndCtrl will not block
	copy(sw.ack.SendGUID, s.GUID)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.ack.GUID)
	sw.buffer.Write(sw.ack.RoleGUID)
	sw.buffer.Write(sw.ack.SendGUID)
	sw.ack.Signature = sw.ctx.global.Sign(sw.buffer.Bytes())
	// encode
	sw.buffer.Reset()
	sw.err = sw.encoder.Encode(sw.ack)
	if sw.err != nil {
		panic(sw.err)
	}
	sw.ctx.forwarder.AckToNodeAndCtrl(sw.ack.GUID, sw.buffer.Bytes(), "")
}

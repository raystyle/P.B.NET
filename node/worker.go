package node

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"hash"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

// worker is used to handle message from controller
type worker struct {
	broadcastQueue chan *protocol.Broadcast
	sendQueue      chan *protocol.Send
	ackQueue       chan *protocol.Acknowledge

	broadcastPool sync.Pool
	sendPool      sync.Pool
	ackPool       sync.Pool

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
		broadcastQueue: make(chan *protocol.Broadcast, cfg.QueueSize),
		sendQueue:      make(chan *protocol.Send, cfg.QueueSize),
		ackQueue:       make(chan *protocol.Acknowledge, cfg.QueueSize),
		stopSignal:     make(chan struct{}),
	}

	worker.broadcastPool.New = func() interface{} {
		return &protocol.Broadcast{
			GUID:      make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	worker.sendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	worker.ackPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}

	// start sub workers
	broadcastPoolP := &worker.broadcastPool
	sendPoolP := &worker.sendPool
	ackPoolP := &worker.ackPool
	wgP := &worker.wg
	worker.wg.Add(cfg.Number)
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx:            ctx,
			maxBufferSize:  cfg.MaxBufferSize,
			broadcastQueue: worker.broadcastQueue,
			sendQueue:      worker.sendQueue,
			ackQueue:       worker.ackQueue,
			broadcastPool:  broadcastPoolP,
			sendPool:       sendPoolP,
			ackPool:        ackPoolP,
			stopSignal:     worker.stopSignal,
			wg:             wgP,
		}
		go sw.Work()
	}
	return &worker, nil
}

// GetBroadcastFromPool is used to get *protocol.Broadcast from broadcastPool
func (ws *worker) GetBroadcastFromPool() *protocol.Broadcast {
	return ws.broadcastPool.Get().(*protocol.Broadcast)
}

// PutBroadcastToPool is used to put *protocol.Broadcast to broadcastPool
func (ws *worker) PutBroadcastToPool(b *protocol.Broadcast) {
	ws.broadcastPool.Put(b)
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

// AddBroadcast is used to add broadcast to sub workers
func (ws *worker) AddBroadcast(b *protocol.Broadcast) {
	select {
	case ws.broadcastQueue <- b:
	case <-ws.stopSignal:
	}
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
	ackQueue       chan *protocol.Acknowledge
	broadcastPool  *sync.Pool
	sendPool       *sync.Pool
	ackPool        *sync.Pool

	// runtime
	buffer  *bytes.Buffer
	hash    hash.Hash
	hex     io.Writer
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
			sw.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.hash = sha256.New()
	sw.hex = hex.NewEncoder(sw.buffer)
	sw.ack = &protocol.Acknowledge{SendGUID: make([]byte, guid.Size)}
	sw.encoder = msgpack.NewEncoder(sw.buffer)
	var (
		b *protocol.Broadcast
		s *protocol.Send
		a *protocol.Acknowledge
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
		case a = <-sw.ackQueue:
			sw.handleAcknowledge(a)
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
	sw.ctx.sender.Acknowledge(s)
	sw.ctx.handler.OnSend(s)
}

func (sw *subWorker) handleAcknowledge(a *protocol.Acknowledge) {
	defer sw.ackPool.Put(a)
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(a.GUID)
	sw.buffer.Write(a.RoleGUID)
	sw.buffer.Write(a.SendGUID)
	if !sw.ctx.global.CtrlVerify(sw.buffer.Bytes(), a.Signature) {
		const format = "invalid acknowledge signature\nGUID: %X"
		sw.logf(logger.Exploit, format, a.GUID)
		return
	}
	sw.buffer.Reset()
	_, _ = sw.hex.Write(a.SendGUID)
	sw.ctx.sender.HandleAcknowledge(sw.buffer.String())
}

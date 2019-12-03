package controller

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"hash"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type worker struct {
	nodeSendQueue   chan *protocol.Send
	BeaconSendQueue chan *protocol.Send
	queryQueue      chan *protocol.Query

	sendPool  sync.Pool
	queryPool sync.Pool

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newWorker(ctx *CTRL, config *Config) (*worker, error) {
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
		nodeSendQueue:   make(chan *protocol.Send, cfg.QueueSize),
		BeaconSendQueue: make(chan *protocol.Send, cfg.QueueSize),
		queryQueue:      make(chan *protocol.Query, cfg.QueueSize),
		stopSignal:      make(chan struct{}),
	}
	worker.sendPool.New = func() interface{} {
		return new(protocol.Send)
	}
	worker.queryPool.New = func() interface{} {
		return new(protocol.Query)
	}

	// start sub workers
	sendPoolP := &worker.sendPool
	queryPoolP := &worker.queryPool
	wgP := &worker.wg
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx: ctx,

			maxBufferSize: cfg.MaxBufferSize,

			nodeSendQueue:   worker.nodeSendQueue,
			BeaconSendQueue: worker.BeaconSendQueue,
			queryQueue:      worker.queryQueue,
			sendPool:        sendPoolP,
			queryPool:       queryPoolP,

			stopSignal: worker.stopSignal,
			wg:         wgP,
		}
		worker.wg.Add(1)
		go sw.Work()
	}
	return &worker, nil
}

// GetSendFromPool is used to get *protocol.Send from sendPool
func (ws *worker) GetSendFromPool() *protocol.Send {
	return ws.sendPool.Get().(*protocol.Send)
}

// GetQueryFromPool is used to get *protocol.Query from queryPool
func (ws *worker) GetQueryFromPool() *protocol.Query {
	return ws.queryPool.Get().(*protocol.Query)
}

// PutSendToPool is used to put *protocol.Send to sendPool
func (ws *worker) PutSendToPool(s *protocol.Send) {
	ws.sendPool.Put(s)
}

// PutQueryToPool is used to put *protocol.Query to queryPool
func (ws *worker) PutQueryToPool(q *protocol.Query) {
	ws.queryPool.Put(q)
}

// AddNodeSend is used to add node send to sub workers
func (ws *worker) AddNodeSend(s *protocol.Send) {
	select {
	case ws.nodeSendQueue <- s:
	case <-ws.stopSignal:
	}
}

// AddBeaconSend is used to add beacon send to sub workers
func (ws *worker) AddBeaconSend(s *protocol.Send) {
	select {
	case ws.BeaconSendQueue <- s:
	case <-ws.stopSignal:
	}
}

// AddQuery is used to add query to sub workers
func (ws *worker) AddQuery(q *protocol.Query) {
	select {
	case ws.queryQueue <- q:
	case <-ws.stopSignal:
	}
}

// Close is used to close all sub workers
func (ws *worker) Close() {
	close(ws.stopSignal)
	ws.wg.Wait()
}

type subWorker struct {
	ctx *CTRL

	maxBufferSize int

	nodeSendQueue   chan *protocol.Send
	BeaconSendQueue chan *protocol.Send
	queryQueue      chan *protocol.Query
	sendPool        *sync.Pool
	queryPool       *sync.Pool

	// runtime
	buffer    *bytes.Buffer
	hash      hash.Hash
	node      *mNode
	beacon    *mBeacon
	publicKey ed25519.PublicKey
	aesKey    []byte
	aesIV     []byte
	err       error

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
	var (
		s *protocol.Send
		q *protocol.Query
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
		case s = <-sw.nodeSendQueue:
			sw.handleNodeSend(s)
		case s = <-sw.BeaconSendQueue:
			sw.handleBeaconSend(s)
		case q = <-sw.queryQueue:
			sw.handleQuery(q)
		case <-sw.stopSignal:
			return
		}
	}
}

func (sw *subWorker) handleNodeSend(s *protocol.Send) {
	defer sw.sendPool.Put(s)
	// set key
	sw.node, sw.err = sw.ctx.db.SelectNode(s.RoleGUID)
	if sw.err != nil {
		const format = "failed to select node: %s\nGUID: %X"
		sw.logf(logger.Warning, format, sw.err, s.RoleGUID)
		return
	}
	sw.publicKey = sw.node.PublicKey
	sw.aesKey = sw.node.SessionKey
	sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(s.GUID)
	sw.buffer.Write(s.RoleGUID)
	sw.buffer.Write(s.Message)
	sw.buffer.Write(s.Hash)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), s.Signature) {
		const format = "invalid node send signature\nGUID: %X"
		sw.logf(logger.Exploit, format, s.RoleGUID)
		return
	}
	// decrypt
	s.Message, sw.err = aes.CBCDecrypt(s.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		const format = "failed to decrypt node send: %s\nGUID: %X"
		sw.logf(logger.Exploit, format, sw.err, s.RoleGUID)
		return
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(s.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), s.Hash) != 1 {
		const format = "node send with incorrect hash\nGUID: %X"
		sw.logf(logger.Exploit, format, s.RoleGUID)
		return
	}
	sw.ctx.handler.OnNodeSend(s)
	sw.ctx.sender.Acknowledge(protocol.Node, s)
}

func (sw *subWorker) handleBeaconSend(s *protocol.Send) {
	defer sw.sendPool.Put(s)
	// set key
	sw.beacon, sw.err = sw.ctx.db.SelectBeacon(s.RoleGUID)
	if sw.err != nil {
		const format = "failed to select beacon: %s\nGUID: %X"
		sw.logf(logger.Warning, format, sw.err, s.RoleGUID)
		return
	}
	sw.publicKey = sw.beacon.PublicKey
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(s.GUID)
	sw.buffer.Write(s.RoleGUID)
	sw.buffer.Write(s.Message)
	sw.buffer.Write(s.Hash)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), s.Signature) {
		const format = "invalid beacon send signature\nGUID: %X"
		sw.logf(logger.Exploit, format, s.RoleGUID)
		return
	}
	// decrypt
	s.Message, sw.err = aes.CBCDecrypt(s.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		const format = "failed to decrypt beacon send: %s\nGUID: %X"
		sw.logf(logger.Exploit, format, sw.err, s.RoleGUID)
		return
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(s.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), s.Hash) != 1 {
		const format = "beacon send with incorrect hash\nGUID: %X"
		sw.logf(logger.Exploit, format, s.RoleGUID)
		return
	}
	sw.ctx.handler.OnBeaconSend(s)
	sw.ctx.sender.Acknowledge(protocol.Beacon, s)
}

func (sw *subWorker) handleQuery(q *protocol.Query) {
	defer sw.queryPool.Put(q)
	// set key
	sw.beacon, sw.err = sw.ctx.db.SelectBeacon(q.BeaconGUID)
	if sw.err != nil {
		const format = "failed to select beacon: %s\nGUID: %X"
		sw.logf(logger.Warning, format, sw.err, q.BeaconGUID)
		return
	}
	sw.publicKey = sw.beacon.PublicKey
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(q.GUID)
	sw.buffer.Write(q.BeaconGUID)
	sw.buffer.Write(convert.Uint64ToBytes(q.Index))
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), q.Signature) {
		const format = "invalid query signature\nGUID: %X"
		sw.logf(logger.Exploit, format, q.BeaconGUID)
		return
	}
	// query message

	// TODO query message and answer
	// may be copy send
}

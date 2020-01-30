package controller

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

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type worker struct {
	nodeSendQueue   chan *protocol.Send
	beaconSendQueue chan *protocol.Send
	nodeAckQueue    chan *protocol.Acknowledge
	beaconAckQueue  chan *protocol.Acknowledge
	queryQueue      chan *protocol.Query

	sendPool  sync.Pool
	ackPool   sync.Pool
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
	if cfg.MaxBufferSize < 16<<10 {
		return nil, errors.New("worker max buffer size must >= 16KB")
	}

	worker := worker{
		nodeSendQueue:   make(chan *protocol.Send, cfg.QueueSize),
		beaconSendQueue: make(chan *protocol.Send, cfg.QueueSize),
		nodeAckQueue:    make(chan *protocol.Acknowledge, cfg.QueueSize),
		beaconAckQueue:  make(chan *protocol.Acknowledge, cfg.QueueSize),
		queryQueue:      make(chan *protocol.Query, cfg.QueueSize),
		stopSignal:      make(chan struct{}),
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
	worker.queryPool.New = func() interface{} {
		return &protocol.Query{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}

	// start sub workers
	sendPoolP := &worker.sendPool
	ackPoolP := &worker.ackPool
	queryPoolP := &worker.queryPool
	wgP := &worker.wg
	worker.wg.Add(cfg.Number)
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx:             ctx,
			maxBufferSize:   cfg.MaxBufferSize,
			nodeSendQueue:   worker.nodeSendQueue,
			beaconSendQueue: worker.beaconSendQueue,
			nodeAckQueue:    worker.nodeAckQueue,
			beaconAckQueue:  worker.beaconAckQueue,
			queryQueue:      worker.queryQueue,
			sendPool:        sendPoolP,
			ackPool:         ackPoolP,
			queryPool:       queryPoolP,
			stopSignal:      worker.stopSignal,
			wg:              wgP,
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

// GetQueryFromPool is used to get *protocol.Query from queryPool
func (ws *worker) GetQueryFromPool() *protocol.Query {
	return ws.queryPool.Get().(*protocol.Query)
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
	case ws.beaconSendQueue <- s:
	case <-ws.stopSignal:
	}
}

// AddNodeAcknowledge is used to add node acknowledge to sub workers
func (ws *worker) AddNodeAcknowledge(a *protocol.Acknowledge) {
	select {
	case ws.nodeAckQueue <- a:
	case <-ws.stopSignal:
	}
}

// AddBeaconAcknowledge is used to add beacon acknowledge to sub workers
func (ws *worker) AddBeaconAcknowledge(a *protocol.Acknowledge) {
	select {
	case ws.beaconAckQueue <- a:
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

	// copy from worker
	nodeSendQueue   chan *protocol.Send
	beaconSendQueue chan *protocol.Send
	nodeAckQueue    chan *protocol.Acknowledge
	beaconAckQueue  chan *protocol.Acknowledge
	queryQueue      chan *protocol.Query

	sendPool  *sync.Pool
	ackPool   *sync.Pool
	queryPool *sync.Pool

	// runtime
	buffer    *bytes.Buffer
	hash      hash.Hash
	hex       io.Writer
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
	sw.hex = hex.NewEncoder(sw.buffer)
	var (
		send  *protocol.Send
		ack   *protocol.Acknowledge
		query *protocol.Query
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
		case send = <-sw.nodeSendQueue:
			sw.handleNodeSend(send)
		case send = <-sw.beaconSendQueue:
			sw.handleBeaconSend(send)
		case ack = <-sw.nodeAckQueue:
			sw.handleNodeAcknowledge(ack)
		case ack = <-sw.beaconAckQueue:
			sw.handleBeaconAcknowledge(ack)
		case query = <-sw.queryQueue:
			sw.handleQuery(query)
		case <-sw.stopSignal:
			return
		}
	}
}

func (sw *subWorker) getNodeKey(guid []byte) bool {
	sw.node, sw.err = sw.ctx.database.SelectNode(guid)
	if sw.err != nil {
		const format = "failed to select node: %s\nGUID: %X"
		sw.logf(logger.Warning, format, sw.err, guid)
		return false
	}
	sw.publicKey = sw.node.PublicKey
	sw.aesKey = sw.node.SessionKey
	sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	return true
}

func (sw *subWorker) getBeaconKey(guid []byte) bool {
	sw.beacon, sw.err = sw.ctx.database.SelectBeacon(guid)
	if sw.err != nil {
		const format = "failed to select beacon: %s\nGUID: %X"
		sw.logf(logger.Warning, format, sw.err, guid)
		return false
	}
	sw.publicKey = sw.beacon.PublicKey
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	return true
}

// return cache
func (sw *subWorker) handleRoleSend(role protocol.Role, send *protocol.Send) []byte {
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(send.GUID)
	sw.buffer.Write(send.RoleGUID)
	sw.buffer.Write(send.Message)
	sw.buffer.Write(send.Hash)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), send.Signature) {
		const format = "invalid %s send signature\nGUID: %X"
		sw.logf(logger.Exploit, format, role, send.RoleGUID)
		return nil
	}
	// decrypt message
	cache := send.Message
	send.Message, sw.err = aes.CBCDecrypt(send.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		const format = "failed to decrypt %s send: %s\nGUID: %X"
		sw.logf(logger.Exploit, format, role, sw.err, send.RoleGUID)
		return nil
	}
	// compare hash
	sw.hash.Reset()
	sw.hash.Write(send.Message)
	if subtle.ConstantTimeCompare(sw.hash.Sum(nil), send.Hash) != 1 {
		const format = "%s send with incorrect hash\nGUID: %X"
		sw.logf(logger.Exploit, format, role, send.RoleGUID)
		return nil
	}
	return cache
}

func (sw *subWorker) handleNodeSend(send *protocol.Send) {
	defer sw.sendPool.Put(send)
	if !sw.getNodeKey(send.RoleGUID) {
		return
	}
	cache := sw.handleRoleSend(protocol.Node, send)
	if cache == nil {
		return
	}
	defer func() { send.Message = cache }()
	sw.ctx.sender.Acknowledge(protocol.Node, send)
	sw.ctx.handler.OnNodeSend(send)
}

func (sw *subWorker) handleBeaconSend(send *protocol.Send) {
	defer sw.sendPool.Put(send)
	if !sw.getBeaconKey(send.RoleGUID) {
		return
	}
	cache := sw.handleRoleSend(protocol.Beacon, send)
	if cache == nil {
		return
	}
	defer func() { send.Message = cache }()
	sw.ctx.sender.Acknowledge(protocol.Beacon, send)
	sw.ctx.handler.OnBeaconSend(send)
}

func (sw *subWorker) verifyAcknowledge(role protocol.Role, ack *protocol.Acknowledge) bool {
	sw.buffer.Reset()
	sw.buffer.Write(ack.GUID)
	sw.buffer.Write(ack.RoleGUID)
	sw.buffer.Write(ack.SendGUID)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), ack.Signature) {
		const format = "invalid %s acknowledge signature\nGUID: %X"
		sw.logf(logger.Exploit, format, role, ack.RoleGUID)
		return false
	}
	return true
}

func (sw *subWorker) handleAcknowledge(role protocol.Role, ack *protocol.Acknowledge) {
	sw.buffer.Reset()
	_, _ = sw.hex.Write(ack.RoleGUID)
	roleGUID := sw.buffer.String()
	sw.buffer.Reset()
	_, _ = sw.hex.Write(ack.SendGUID)
	switch role {
	case protocol.Node:
		sw.ctx.sender.HandleNodeAcknowledge(roleGUID, sw.buffer.String())
	case protocol.Beacon:
		sw.ctx.sender.HandleBeaconAcknowledge(roleGUID, sw.buffer.String())
	}
}

func (sw *subWorker) handleNodeAcknowledge(ack *protocol.Acknowledge) {
	defer sw.ackPool.Put(ack)
	if !sw.getNodeKey(ack.RoleGUID) {
		return
	}
	if !sw.verifyAcknowledge(protocol.Node, ack) {
		return
	}
	sw.handleAcknowledge(protocol.Node, ack)
}

func (sw *subWorker) handleBeaconAcknowledge(ack *protocol.Acknowledge) {
	defer sw.ackPool.Put(ack)
	if !sw.getBeaconKey(ack.RoleGUID) {
		return
	}
	if !sw.verifyAcknowledge(protocol.Beacon, ack) {
		return
	}
	sw.handleAcknowledge(protocol.Beacon, ack)
}

func (sw *subWorker) handleQuery(query *protocol.Query) {
	defer sw.queryPool.Put(query)
	if !sw.getBeaconKey(query.BeaconGUID) {
		return
	}
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(query.GUID)
	sw.buffer.Write(query.BeaconGUID)
	sw.buffer.Write(convert.Uint64ToBytes(query.Index))
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), query.Signature) {
		const format = "invalid query signature\nGUID: %X"
		sw.logf(logger.Exploit, format, query.BeaconGUID)
		return
	}
	// query message

	// TODO query message and answer
	// may be copy send
}

package controller

import (
	"bytes"
	"compress/flate"
	"crypto/subtle"
	"hash"
	"io"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/crypto/aes"
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

func newWorker(ctx *Ctrl, config *Config) (*worker, error) {
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
		return protocol.NewSend()
	}
	worker.ackPool.New = func() interface{} {
		return protocol.NewAcknowledge()
	}
	worker.queryPool.New = func() interface{} {
		return protocol.NewQuery()
	}

	// start sub workers
	sendPoolP := &worker.sendPool
	ackPoolP := &worker.ackPool
	queryPoolP := &worker.queryPool
	wgP := &worker.wg
	worker.wg.Add(2 * cfg.Number)
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
		go sw.WorkWithBlock()
	}
	for i := 0; i < cfg.Number; i++ {
		sw := subWorker{
			ctx:            ctx,
			maxBufferSize:  cfg.MaxBufferSize,
			nodeAckQueue:   worker.nodeAckQueue,
			beaconAckQueue: worker.beaconAckQueue,
			ackPool:        ackPoolP,
			stopSignal:     worker.stopSignal,
			wg:             wgP,
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
	ctx *Ctrl

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
	buffer  *bytes.Buffer
	reader  *bytes.Reader
	deflate io.ReadCloser

	// key
	node      *mNode
	beacon    *mBeacon
	aesKey    []byte
	aesIV     []byte
	hmac      hash.Hash
	beaconMsg *mBeaconMessage
	timer     *time.Timer
	err       error

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
	// must stop at once, or maybe timeout at the first time.
	sw.timer = time.NewTimer(time.Minute)
	sw.timer.Stop()
	defer sw.timer.Stop()
	var (
		send        *protocol.Send
		acknowledge *protocol.Acknowledge
		query       *protocol.Query
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
		case acknowledge = <-sw.nodeAckQueue:
			sw.handleNodeAcknowledge(acknowledge)
		case acknowledge = <-sw.beaconAckQueue:
			sw.handleBeaconAcknowledge(acknowledge)
		case query = <-sw.queryQueue:
			sw.handleQuery(query)
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
		case acknowledge = <-sw.nodeAckQueue:
			sw.handleNodeAcknowledge(acknowledge)
		case acknowledge = <-sw.beaconAckQueue:
			sw.handleBeaconAcknowledge(acknowledge)
		case <-sw.stopSignal:
			return
		}
	}
}

func (sw *subWorker) getNodeKey(guid *guid.GUID, session bool) bool {
	sw.node, sw.err = sw.ctx.database.SelectNode(guid)
	if sw.err != nil {
		const format = "failed to select node: %s\n%s"
		sw.logf(logger.Warning, format, sw.err, guid.Print())
		return false
	}
	sw.hmac = sw.node.HMACPool.Get().(hash.Hash)
	if session {
		sw.aesKey = sw.node.SessionKey.Get()
		sw.aesIV = sw.aesKey[:aes.IVSize]
	}
	return true
}

func (sw *subWorker) getBeaconKey(guid *guid.GUID, session bool) bool {
	sw.beacon, sw.err = sw.ctx.database.SelectBeacon(guid)
	if sw.err != nil {
		const format = "failed to select beacon: %s\n%s"
		sw.logf(logger.Warning, format, sw.err, guid.Print())
		return false
	}
	sw.hmac = sw.beacon.HMACPool.Get().(hash.Hash)
	if session {
		sw.aesKey = sw.beacon.SessionKey.Get()
		sw.aesIV = sw.aesKey[:aes.IVSize]
	}
	return true
}

func (sw *subWorker) handleNodeSend(send *protocol.Send) {
	defer sw.sendPool.Put(send)
	if !sw.getNodeKey(&send.RoleGUID, true) {
		return
	}
	aesBuffer := sw.handleNodeSendDefer(send)
	if aesBuffer == nil {
		return
	}
	defer func() {
		if send.Deflate == 1 {
			send.Message = aesBuffer
		}
	}()
	sw.ctx.handler.OnNodeSend(send)
	for {
		sw.err = sw.ctx.sender.AckToNode(send)
		if sw.err == nil {
			return
		}
		if sw.err == ErrNoConnections || sw.err == ErrFailedToAckToNode {
			sw.log(logger.Warning, "failed to ack to node:", sw.err)
		} else {
			sw.log(logger.Error, "failed to ack to node:", sw.err)
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

func (sw *subWorker) handleNodeSendDefer(send *protocol.Send) []byte {
	defer func() {
		sw.node.SessionKey.Put(sw.aesKey)
		sw.node.HMACPool.Put(sw.hmac)
	}()
	return sw.handleRoleSend(protocol.Node, send)
}

func (sw *subWorker) handleBeaconSend(send *protocol.Send) {
	defer sw.sendPool.Put(send)
	if !sw.getBeaconKey(&send.RoleGUID, true) {
		return
	}
	aesBuffer := sw.handleBeaconSendDefer(send)
	if aesBuffer == nil {
		return
	}
	defer func() {
		if send.Deflate == 1 {
			send.Message = aesBuffer
		}
	}()
	sw.ctx.handler.OnBeaconSend(send)
	for {
		sw.err = sw.ctx.sender.AckToBeacon(send)
		if sw.err == nil {
			return
		}
		if sw.err == ErrNoConnections || sw.err == ErrFailedToAckToBeacon {
			sw.log(logger.Warning, "failed to ack to beacon:", sw.err)
		} else {
			sw.log(logger.Error, "failed to ack to beacon:", sw.err)
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

func (sw *subWorker) handleBeaconSendDefer(send *protocol.Send) []byte {
	defer func() {
		sw.beacon.SessionKey.Put(sw.aesKey)
		sw.beacon.HMACPool.Put(sw.hmac)
	}()
	return sw.handleRoleSend(protocol.Beacon, send)
}

// return aesBuffer
func (sw *subWorker) handleRoleSend(role protocol.Role, send *protocol.Send) []byte {
	// verify
	if subtle.ConstantTimeCompare(sw.calculateRoleSendHMAC(send), send.Hash) != 1 {
		const format = "%s send with incorrect hmac hash\n%s"
		sw.logf(logger.Exploit, format, role, spew.Sdump(send))
		return nil
	}
	// decrypt message
	send.Message, sw.err = aes.CBCDecrypt(send.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		const format = "failed to decrypt %s send: %s\n%s"
		sw.logf(logger.Exploit, format, role, sw.err, spew.Sdump(send))
		return nil
	}
	// must recover it, otherwise will appear data race
	aesBuffer := send.Message
	// decompress message
	if send.Deflate == 1 {
		sw.reader.Reset(send.Message)
		sw.err = sw.deflate.(flate.Resetter).Reset(sw.reader, nil)
		if sw.err != nil {
			const format = "failed to reset deflate reader about %s send: %s\n%s"
			sw.logf(logger.Exploit, format, role, sw.err, spew.Sdump(send))
			return nil
		}
		sw.buffer.Reset()
		_, sw.err = sw.buffer.ReadFrom(sw.deflate)
		if sw.err != nil {
			const format = "failed to decompress %s send: %s\n%s"
			sw.logf(logger.Exploit, format, role, sw.err, spew.Sdump(send))
			return nil
		}
		sw.err = sw.deflate.Close()
		if sw.err != nil {
			const format = "failed to close deflate reader about %s send: %s\n%s"
			sw.logf(logger.Exploit, format, role, sw.err, spew.Sdump(send))
			return nil
		}
		send.Message = sw.buffer.Bytes()
	}
	return aesBuffer
}

func (sw *subWorker) calculateRoleSendHMAC(send *protocol.Send) []byte {
	sw.hmac.Reset()
	sw.hmac.Write(send.GUID[:])
	sw.hmac.Write(send.RoleGUID[:])
	sw.hmac.Write([]byte{send.Deflate})
	sw.hmac.Write(send.Message)
	return sw.hmac.Sum(nil)
}

func (sw *subWorker) handleNodeAcknowledge(ack *protocol.Acknowledge) {
	defer sw.ackPool.Put(ack)
	if !sw.getNodeKey(&ack.RoleGUID, false) {
		return
	}
	defer sw.node.HMACPool.Put(sw.hmac)
	// verify
	if subtle.ConstantTimeCompare(sw.calculateRoleAcknowledgeHMAC(ack), ack.Hash) != 1 {
		const format = "node acknowledge with incorrect hmac hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(ack))
		return
	}
	sw.ctx.sender.HandleNodeAcknowledge(&ack.RoleGUID, &ack.SendGUID)
}

func (sw *subWorker) handleBeaconAcknowledge(ack *protocol.Acknowledge) {
	defer sw.ackPool.Put(ack)
	if !sw.getBeaconKey(&ack.RoleGUID, false) {
		return
	}
	defer sw.beacon.HMACPool.Put(sw.hmac)
	// verify
	if subtle.ConstantTimeCompare(sw.calculateRoleAcknowledgeHMAC(ack), ack.Hash) != 1 {
		const format = "beacon acknowledge with incorrect hmac hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(ack))
		return
	}
	sw.ctx.sender.HandleBeaconAcknowledge(&ack.RoleGUID, &ack.SendGUID)
}

func (sw *subWorker) calculateRoleAcknowledgeHMAC(ack *protocol.Acknowledge) []byte {
	sw.hmac.Reset()
	sw.hmac.Write(ack.GUID[:])
	sw.hmac.Write(ack.RoleGUID[:])
	sw.hmac.Write(ack.SendGUID[:])
	return sw.hmac.Sum(nil)
}

func (sw *subWorker) handleQuery(query *protocol.Query) {
	defer sw.queryPool.Put(query)
	if !sw.getBeaconKey(&query.BeaconGUID, false) {
		return
	}
	// verify
	if subtle.ConstantTimeCompare(sw.calculateQueryHMAC(query), query.Hash) != 1 {
		const format = "invalid query hmac hash\n%s"
		sw.logf(logger.Exploit, format, spew.Sdump(query))
		return
	}
	// first try to select beacon message
	sw.beaconMsg, sw.err = sw.ctx.database.SelectBeaconMessage(query)
	if sw.err != nil {
		const format = "failed to select beacon message\nerror:%s\n%s"
		sw.logf(logger.Error, format, sw.err, spew.Sdump(query))
		return
	}
	// maybe no message
	if sw.beaconMsg == nil {
		return
	}
	// then delete old message
	sw.err = sw.ctx.database.DeleteBeaconMessage(query)
	if sw.err != nil {
		const format = "failed to delete old beacon message\nerror: %s\n%s"
		sw.logf(logger.Error, format, sw.err, spew.Sdump(query))
		return
	}
	for {
		sw.err = sw.ctx.sender.Answer(sw.beaconMsg)
		if sw.err == nil {
			return
		}
		if sw.err == ErrNoConnections || sw.err == ErrFailedToAnswer {
			sw.log(logger.Warning, "failed answer in handle query:", sw.err)
		} else {
			sw.log(logger.Error, "failed answer in handle query:", sw.err)
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

func (sw *subWorker) calculateQueryHMAC(query *protocol.Query) []byte {
	defer sw.beacon.HMACPool.Put(sw.hmac)
	sw.hmac.Reset()
	sw.hmac.Write(query.GUID[:])
	sw.hmac.Write(query.BeaconGUID[:])
	sw.hmac.Write(convert.BEUint64ToBytes(query.Index))
	return sw.hmac.Sum(nil)
}

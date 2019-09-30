package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

const (
	senderNode   = 0
	senderBeacon = 1
)

var (
	errBroadcastFailed = errors.New("broadcast failed")
	errSendFailed      = errors.New("send failed")
)

type broadcastTask struct {
	Type     []byte // message type
	MessageI interface{}
	Message  []byte // Message include message type
	Result   chan<- *protocol.BroadcastResult
}

type sendTask struct {
	Role     protocol.Role // receiver role
	GUID     []byte        // receiver role's GUID
	Type     []byte        // message type
	MessageI interface{}
	Message  []byte // Message include message type
	Result   chan<- *protocol.SendResult
}

type acknowledgeTask struct {
	Role     protocol.Role
	GUID     []byte
	SendGUID []byte
}

type sender struct {
	ctx *CTRL

	broadcastTaskQueue   chan *broadcastTask
	sendTaskQueue        chan *sendTask
	acknowledgeTaskQueue chan *acknowledgeTask

	broadcastTaskPool   sync.Pool
	sendTaskPool        sync.Pool
	acknowledgeTaskPool sync.Pool

	broadcastDonePool sync.Pool
	sendDonePool      sync.Pool

	broadcastResultPool sync.Pool
	sendResultPool      sync.Pool

	broadcastRespPool sync.Pool
	sendRespPool      sync.Pool
	waitGroupPool     sync.Pool

	// check is sending
	sendStatus    [2]map[string]bool // key = base64(Role GUID)
	sendStatusRWM [2]sync.RWMutex

	// check beacon is in interactive mode
	interactive    map[string]bool // key = base64(Beacon GUID)
	interactiveRWM sync.RWMutex

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSender(ctx *CTRL, cfg *Config) (*sender, error) {
	// check config
	if cfg.SenderWorker < 1 {
		return nil, errors.New("sender worker number < 1")
	}
	if cfg.SenderQueueSize < 512 {
		return nil, errors.New("sender task queue size < 512")
	}
	sender := sender{
		ctx:                  ctx,
		broadcastTaskQueue:   make(chan *broadcastTask, cfg.SenderQueueSize),
		sendTaskQueue:        make(chan *sendTask, cfg.SenderQueueSize),
		acknowledgeTaskQueue: make(chan *acknowledgeTask, cfg.SenderQueueSize),
		stopSignal:           make(chan struct{}),
	}
	sender.sendStatus[senderNode] = make(map[string]bool)
	sender.sendStatus[senderBeacon] = make(map[string]bool)
	// init task sync pool
	sender.broadcastTaskPool.New = func() interface{} {
		return new(broadcastTask)
	}
	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.acknowledgeTaskPool.New = func() interface{} {
		return new(acknowledgeTask)
	}
	// init done sync pool
	sender.broadcastDonePool.New = func() interface{} {
		return make(chan *protocol.BroadcastResult, 1)
	}
	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	// init result sync pool
	sender.broadcastResultPool.New = func() interface{} {
		return new(protocol.BroadcastResult)
	}
	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	// init response sync pool
	sender.broadcastRespPool.New = func() interface{} {
		return make(chan *protocol.BroadcastResponse, 1)
	}
	sender.sendRespPool.New = func() interface{} {
		return make(chan *protocol.SendResponse, 1)
	}
	// init wait group sync pool
	sender.waitGroupPool.New = func() interface{} {
		return sync.WaitGroup{}
	}
	// start sender workers
	for i := 0; i < cfg.SenderWorker; i++ {
		worker := senderWorker{
			ctx:           &sender,
			maxBufferSize: cfg.MaxBufferSize,
		}
		sender.wg.Add(1)
		go worker.Work()
	}
	return &sender, nil
}

// Broadcast is used to broadcast message to all Nodes
// message will not be saved
func (sender *sender) Broadcast(Type []byte, message interface{}) error {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	bt.Type = Type
	bt.MessageI = message
	bt.Result = done
	sender.broadcastTaskQueue <- bt
	result := <-done
	// record
	err := result.Err
	// put
	sender.broadcastDonePool.Put(done)
	sender.broadcastTaskPool.Put(bt)
	sender.broadcastResultPool.Put(result)
	return err
}

// Broadcast is used to broadcast(plugin) message to all Nodes
func (sender *sender) BroadcastPlugin(message []byte) error {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	bt.Message = message
	bt.Result = done
	sender.broadcastTaskQueue <- bt
	result := <-done
	// record
	err := result.Err
	// put
	sender.broadcastDonePool.Put(done)
	sender.broadcastTaskPool.Put(bt)
	sender.broadcastResultPool.Put(result)
	return err
}

// Send is used to send message to Node or Beacon
//
// if Beacon is not in interactive mode,
// message will saved to database, Beacon will query it.
// Node is always in interactive mode
func (sender *sender) Send(
	role protocol.Role,
	guid,
	Type []byte,
	message interface{},
) (r *protocol.SendResult) {
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	st := sender.sendTaskPool.Get().(*sendTask)
	st.Role = role
	st.GUID = guid
	st.Type = Type
	st.MessageI = message
	st.Result = done
	sender.sendTaskQueue <- st
	r = <-done
	sender.sendDonePool.Put(done)
	return
}

// Send is used to send(async) message to Node or Beacon
func (sender *sender) SendAsync(
	role protocol.Role,
	guid,
	Type []byte,
	message interface{},
	done chan<- *protocol.SendResult,
) {
	st := sender.sendTaskPool.Get().(*sendTask)
	st.Role = role
	st.GUID = guid
	st.Type = Type
	st.MessageI = message
	st.Result = done
	sender.sendTaskQueue <- st
}

// Send is used to send(plugin) message to Node or Beacon
func (sender *sender) SendPlugin(
	role protocol.Role,
	guid,
	message []byte,
) (r *protocol.SendResult) {
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	st := sender.sendTaskPool.Get().(*sendTask)
	st.Role = role
	st.GUID = guid
	st.Message = message
	st.Result = done
	sender.sendTaskQueue <- st
	r = <-done
	sender.sendDonePool.Put(done)
	return
}

// Acknowledge is used to acknowledge Role that controller
// has receive this message in interactive mode
func (sender *sender) Acknowledge(
	role protocol.Role,
	guid []byte,
	sendGUID []byte,
) {
	at := sender.acknowledgeTaskPool.Get().(*acknowledgeTask)
	at.Role = role
	at.GUID = guid
	at.SendGUID = sendGUID
	sender.acknowledgeTaskQueue <- at
}

func (sender *sender) Close() {
	close(sender.stopSignal)
	sender.wg.Wait()
}

func (sender *sender) logf(l logger.Level, format string, log ...interface{}) {
	sender.ctx.Printf(l, "sender", format, log...)
}

func (sender *sender) log(l logger.Level, log ...interface{}) {
	sender.ctx.Print(l, "sender", log...)
}

func (sender *sender) logln(l logger.Level, log ...interface{}) {
	sender.ctx.Println(l, "sender", log...)
}

func (sender *sender) broadcast(token, message []byte) (
	resp []*protocol.BroadcastResponse, success int) {
	sClients := sender.ctx.syncer.Clients()
	l := len(sClients)
	if l == 0 {
		return nil, 0
	}
	// padding channels
	channels := make([]chan *protocol.BroadcastResponse, l)
	for i := 0; i < l; i++ {
		channels[i] = sender.broadcastRespPool.Get().(chan *protocol.BroadcastResponse)
	}
	// broadcast parallel
	index := 0
	for _, sc := range sClients {
		go func(i int, s *sClient) {
			channels[i] <- s.Broadcast(token, message)
		}(index, sc)
		index += 1
	}
	// get response and put
	resp = make([]*protocol.BroadcastResponse, l)
	for i := 0; i < l; i++ {
		resp[i] = <-channels[i]
		if resp[i].Err == nil {
			success += 1
		}
		sender.broadcastRespPool.Put(channels[i])
	}
	return
}

func (sender *sender) send(token, message []byte) (
	resp []*protocol.SendResponse, success int) {
	sClients := sender.ctx.syncer.Clients()
	l := len(sClients)
	if l == 0 {
		return nil, 0
	}
	// padding channels
	channels := make([]chan *protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		channels[i] = sender.sendRespPool.Get().(chan *protocol.SendResponse)
	}
	// send parallel
	index := 0
	for _, sc := range sClients {
		go func(i int, s *sClient) {
			channels[i] <- s.Send(token, message)
		}(index, sc)
		index += 1
	}
	// get response and put
	resp = make([]*protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		resp[i] = <-channels[i]
		if resp[i].Err == nil {
			success += 1
		}
		sender.sendRespPool.Put(channels[i])
	}
	return
}

func (sender *sender) acknowledge(token, message []byte) {
	sClients := sender.ctx.syncer.Clients()
	l := len(sClients)
	if l == 0 {
		return
	}
	// acknowledge parallel
	wg := sender.waitGroupPool.Get().(sync.WaitGroup)
	for _, sc := range sClients {
		wg.Add(1)
		go func(s *sClient) {
			s.Acknowledge(token, message)
			wg.Done()
		}(sc)
	}
	wg.Wait()
	sender.waitGroupPool.Put(wg)
}

// check is sending
func (sender *sender) isSending(role protocol.Role, guid string) bool {
	i := 0
	switch role {
	case protocol.Beacon:
		i = senderBeacon
	case protocol.Node:
		i = senderNode
	}
	sender.sendStatusRWM[i].RLock()
	sending := sender.sendStatus[i][guid]
	sender.sendStatusRWM[i].RUnlock()
	return sending
}

func (sender *sender) sendDone(role protocol.Role, guid string) {
	i := 0
	switch role {
	case protocol.Beacon:
		i = senderBeacon
	case protocol.Node:
		i = senderNode
	}
	sender.sendStatusRWM[i].Lock()
	sender.sendStatus[i][guid] = false
	sender.sendStatusRWM[i].Unlock()
}

func (sender *sender) isInInteractiveMode(guid string) bool {
	sender.interactiveRWM.RLock()
	i := sender.interactive[guid]
	sender.interactiveRWM.RUnlock()
	return i
}

func (sender *sender) SetInteractiveMode(guid string) {
	sender.interactiveRWM.Lock()
	sender.interactive[guid] = true
	sender.interactiveRWM.Unlock()
}

// DeleteRoleStatus is used to delete role send status
// if delete role, must delete it
func (sender *sender) DeleteRoleStatus(role protocol.Role, guid string) {
	i := 0
	switch role {
	case protocol.Beacon:
		i = senderBeacon
		// delete interactive mode
		sender.interactiveRWM.Lock()
		delete(sender.interactive, guid)
		sender.interactiveRWM.Unlock()
	case protocol.Node:
		i = senderNode
	default:
		panic("invalid role")
	}
	sender.sendStatusRWM[i].Lock()
	delete(sender.sendStatus[i], guid)
	sender.sendStatusRWM[i].Unlock()
}

type senderWorker struct {
	ctx           *sender
	maxBufferSize int

	// task
	bt *broadcastTask
	st *sendTask
	at *acknowledgeTask

	// key
	node   *mNode
	beacon *mBeacon
	aesKey []byte
	aesIV  []byte

	// prepare task objects
	preB protocol.Broadcast
	preS protocol.Send
	preA protocol.Acknowledge

	// objects
	guid    *guid.GUID
	buffer  *bytes.Buffer
	msgpack *msgpack.Encoder
	base64  io.WriteCloser
	hash    hash.Hash
	token   []byte

	// temp
	roleGUID string
	err      error
}

func (sw *senderWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("sender.worker() panic:", r)
			sw.ctx.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			sw.ctx.wg.Add(1)
			go sw.Work()
		}
		sw.ctx.wg.Done()
	}()
	// init
	sw.guid = guid.New(16*(runtime.NumCPU()+1), sw.ctx.ctx.global.Now)
	minBufferSize := guid.Size + 9
	sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
	sw.msgpack = msgpack.NewEncoder(sw.buffer)
	sw.base64 = base64.NewEncoder(base64.StdEncoding, sw.buffer)
	sw.hash = sha256.New()
	// token = role + GUID
	sw.token = make([]byte, 1+guid.Size)
	sw.token[0] = protocol.Ctrl.Byte()
	// start handle task
	for {
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		case sw.at = <-sw.ctx.acknowledgeTaskQueue:
			sw.handleAcknowledgeTask()
		case sw.st = <-sw.ctx.sendTaskQueue:
			sw.handleSendTask()
		case sw.bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask()
		case <-sw.ctx.stopSignal:
			return
		}
	}
}

func (sw *senderWorker) handleAcknowledgeTask() {
	defer sw.ctx.acknowledgeTaskPool.Put(sw.at)
	// check role
	if sw.at.Role != protocol.Node && sw.at.Role != protocol.Beacon {
		panic("sender.sender(): invalid at.Role")
	}
	sw.preA.GUID = sw.guid.Get()
	sw.preA.Role = sw.at.Role
	sw.preA.RoleGUID = sw.at.GUID
	sw.preA.SendGUID = sw.at.SendGUID
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preA.GUID)
	sw.buffer.WriteByte(sw.preA.Role.Byte())
	sw.buffer.Write(sw.preA.RoleGUID)
	sw.buffer.Write(sw.preA.SendGUID)
	sw.preA.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack
	sw.buffer.Reset()
	sw.err = sw.msgpack.Encode(sw.preA)
	if sw.err != nil {
		panic(sw.err)
	}
	// set token
	copy(sw.token[1:], sw.preA.GUID)
	sw.ctx.acknowledge(sw.token, sw.buffer.Bytes())
}

func (sw *senderWorker) handleSendTask() {
	result := protocol.SendResult{}
	defer func() {
		sw.st.Result <- &result
	}()
	// check role
	if sw.st.Role != protocol.Node && sw.st.Role != protocol.Beacon {
		result.Err = protocol.ErrInvalidRole
		sw.ctx.sendTaskPool.Put(sw.st)
		return
	}
	// role GUID string
	sw.buffer.Reset()
	_, _ = sw.base64.Write(sw.st.GUID)
	_ = sw.base64.Close()
	sw.roleGUID = sw.buffer.String()
	// check is sending
	if sw.ctx.isSending(sw.st.Role, sw.roleGUID) {
		select {
		case sw.ctx.sendTaskQueue <- sw.st:
		case <-sw.ctx.stopSignal:
		}
		return
	}
	defer sw.ctx.sendDone(sw.st.Role, sw.roleGUID)
	defer sw.ctx.sendTaskPool.Put(sw.st)
	// pack message(interface)
	if sw.st.MessageI != nil {
		sw.buffer.Reset()
		sw.buffer.Write(sw.st.Type)
		result.Err = sw.msgpack.Encode(sw.st.MessageI)
		if result.Err != nil {
			return
		}
		// don't worry copy, because encrypt
		sw.st.Message = sw.buffer.Bytes()
	}
	// check message size
	if len(sw.st.Message) > protocol.MaxMsgSize {
		result.Err = protocol.ErrTooBigMsg
		return
	}
	// set key
	switch sw.st.Role {
	case protocol.Beacon:
		sw.beacon, result.Err = sw.ctx.ctx.db.SelectBeacon(sw.st.GUID)
		if result.Err != nil {
			return
		}
		sw.aesKey = sw.beacon.SessionKey
		sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	case protocol.Node:
		sw.node, result.Err = sw.ctx.ctx.db.SelectNode(sw.st.GUID)
		if result.Err != nil {
			return
		}
		sw.aesKey = sw.node.SessionKey
		sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	default:
		panic("invalid st.Role")
	}
	// encrypt
	sw.preS.Message, result.Err = aes.CBCEncrypt(sw.st.Message, sw.aesKey, sw.aesIV)
	if result.Err != nil {
		return
	}
	// check is need to write message to the database
	if sw.st.Role == protocol.Beacon && !sw.ctx.isInInteractiveMode(sw.roleGUID) {
		result.Err = sw.ctx.ctx.db.InsertBeaconMessage(sw.st.GUID, sw.preS.Message)
		if result.Err == nil {
			result.Success = 1
		}
		return
	}
	// set GUID
	sw.preS.GUID = sw.guid.Get()
	// hash
	sw.hash.Reset()
	sw.hash.Write(sw.st.Message)
	sw.preS.Hash = sw.hash.Sum(nil)
	// role
	sw.preS.Role = sw.st.Role
	sw.preS.RoleGUID = sw.st.GUID
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preS.GUID)
	sw.buffer.Write(sw.preS.Message)
	sw.buffer.Write(sw.preS.Hash)
	sw.buffer.WriteByte(sw.preS.Role.Byte())
	sw.buffer.Write(sw.preS.RoleGUID)
	sw.preS.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack
	sw.buffer.Reset()
	result.Err = sw.msgpack.Encode(sw.preS)
	if result.Err != nil {
		return
	}
	// create acknowledge channel

	// set token
	copy(sw.token[1:], sw.preS.GUID)
	result.Responses, result.Success = sw.ctx.send(sw.token, sw.buffer.Bytes())
	if result.Success == 0 {
		result.Err = errSendFailed
		return
	}
	// wait acknowledge

}

func (sw *senderWorker) handleBroadcastTask() {
	result := sw.ctx.broadcastResultPool.Get().(*protocol.BroadcastResult)
	// must clean
	result.Success = 0
	result.Responses = nil
	result.Err = nil
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("senderWorker.handleBroadcastTask() panic:", r)
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		sw.bt.Result <- result
	}()
	// pack message(interface)
	if sw.bt.MessageI != nil {
		sw.buffer.Reset()
		sw.buffer.Write(sw.bt.Type)
		result.Err = sw.msgpack.Encode(sw.bt.MessageI)
		if result.Err != nil {
			return
		}
		// don't worry copy, because encrypt
		sw.bt.Message = sw.buffer.Bytes()
	}
	// check message size
	if len(sw.bt.Message) > protocol.MaxMsgSize {
		result.Err = protocol.ErrTooBigMsg
		return
	}
	// encrypt
	sw.preB.Message, result.Err = sw.ctx.ctx.global.Encrypt(sw.bt.Message)
	if result.Err != nil {
		return
	}
	// GUID
	sw.preB.GUID = sw.guid.Get()
	// hash
	sw.hash.Reset()
	sw.hash.Write(sw.bt.Message)
	sw.preB.Hash = sw.hash.Sum(nil)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preB.GUID)
	sw.buffer.Write(sw.preB.Message)
	sw.buffer.Write(sw.preB.Hash)
	sw.preB.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack
	sw.buffer.Reset()
	result.Err = sw.msgpack.Encode(sw.preB)
	if result.Err != nil {
		return
	}
	// set token
	copy(sw.token[1:], sw.preB.GUID)
	result.Responses, result.Success = sw.ctx.broadcast(sw.token, sw.buffer.Bytes())
	if result.Success == 0 {
		result.Err = errBroadcastFailed
	}
}

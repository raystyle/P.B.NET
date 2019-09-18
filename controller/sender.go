package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
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

type broadcastTask struct {
	Command  []byte      // for Broadcast
	MessageI interface{} // for Broadcast
	Message  []byte      // for BroadcastPlugin
	Result   chan<- *protocol.BroadcastResult
}

type syncSendTask struct {
	Role     protocol.Role
	Target   []byte
	Command  []byte      // for Send
	MessageI interface{} // for Send
	Message  []byte      // for SendPlugin
	Result   chan<- *protocol.SyncResult
}

type syncReceiveTask struct {
	Role   protocol.Role
	GUID   []byte
	Height uint64
}

type sender struct {
	ctx *CTRL

	broadcastTaskQueue   chan *broadcastTask
	syncSendTaskQueue    chan *syncSendTask
	syncReceiveTaskQueue chan *syncReceiveTask

	broadcastDonePool sync.Pool
	syncSendDonePool  sync.Pool
	broadcastRespPool sync.Pool
	syncRespPool      sync.Pool

	syncSendMs  [2]map[string]*sync.Mutex // role can be only one sync at th same time
	syncSendRWM [2]sync.RWMutex           // key=base64(sender guid) 0=node 1=beacon

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSender(ctx *CTRL, cfg *Config) (*sender, error) {
	// check config
	if cfg.SenderWorker < 1 {
		return nil, errors.New("sender number < 1")
	}
	if cfg.SenderQueueSize < 512 {
		return nil, errors.New("sender task queue size < 512")
	}
	sender := sender{
		ctx:                  ctx,
		broadcastTaskQueue:   make(chan *broadcastTask, cfg.SenderQueueSize),
		syncSendTaskQueue:    make(chan *syncSendTask, cfg.SenderQueueSize),
		syncReceiveTaskQueue: make(chan *syncReceiveTask, cfg.SenderQueueSize),
		stopSignal:           make(chan struct{}),
	}
	sender.syncSendMs[senderNode] = make(map[string]*sync.Mutex)
	sender.syncSendMs[senderBeacon] = make(map[string]*sync.Mutex)
	// init sync pool
	sender.broadcastDonePool.New = func() interface{} {
		return make(chan *protocol.BroadcastResult, 1)
	}
	sender.syncSendDonePool.New = func() interface{} {
		return make(chan *protocol.SyncResult, 1)
	}
	sender.broadcastRespPool.New = func() interface{} {
		return make(chan *protocol.BroadcastResponse, 1)
	}
	sender.syncRespPool.New = func() interface{} {
		return make(chan *protocol.SyncResponse, 1)
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

// Broadcast is used to broadcast message to all nodes
// message will not be saved
func (sender *sender) Broadcast(
	command []byte,
	message interface{},
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	sender.broadcastTaskQueue <- &broadcastTask{
		Command:  command,
		MessageI: message,
		Result:   done,
	}
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

// Broadcast is used to broadcast(Async) message to all nodes
// message will not be saved
func (sender *sender) BroadcastAsync(
	command []byte,
	message interface{},
	done chan<- *protocol.BroadcastResult,
) {
	sender.broadcastTaskQueue <- &broadcastTask{
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

// Broadcast is used to broadcast(plugin) message to all nodes
// message will not be saved
func (sender *sender) BroadcastPlugin(
	message []byte,
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	sender.broadcastTaskQueue <- &broadcastTask{
		Message: message,
		Result:  done,
	}
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

// Send is used to send message to Node or Beacon
// if role not online, node will save it
func (sender *sender) Send(
	role protocol.Role,
	target,
	command []byte,
	message interface{},
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sender.syncSendTaskQueue <- &syncSendTask{
		Role:     role,
		Target:   target,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
	r = <-done
	sender.syncSendDonePool.Put(done)
	return
}

// Send is used to send(async) message to Node or Beacon
// if role not online, node will save it
func (sender *sender) SendAsync(
	role protocol.Role,
	target,
	command []byte,
	message interface{},
	done chan<- *protocol.SyncResult,
) {
	sender.syncSendTaskQueue <- &syncSendTask{
		Role:     role,
		Target:   target,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

// Send is used to send(plugin) message to Node or Beacon
// if role not online, node will save it
func (sender *sender) SendPlugin(
	role protocol.Role,
	target,
	message []byte,
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sender.syncSendTaskQueue <- &syncSendTask{
		Role:    role,
		Target:  target,
		Message: message,
		Result:  done,
	}
	r = <-done
	sender.syncSendDonePool.Put(done)
	return
}

// SyncReceive is used to sync controller receive
// notice node to delete message about Node or Beacon
// only for syncer.worker()
func (sender *sender) SyncReceive(
	role protocol.Role,
	guid []byte,
	height uint64,
) {
	sender.syncReceiveTaskQueue <- &syncReceiveTask{
		Role:   role,
		GUID:   guid,
		Height: height,
	}
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

func (sender *sender) broadcastParallel(token, message []byte) (
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
		go func(s *sClient) {
			channels[index] <- s.Broadcast(token, message)
		}(sc)
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

func (sender *sender) syncSendParallel(token, message []byte) (
	resp []*protocol.SyncResponse, success int) {
	sClients := sender.ctx.syncer.Clients()
	l := len(sClients)
	if l == 0 {
		return nil, 0
	}
	// padding channels
	channels := make([]chan *protocol.SyncResponse, l)
	for i := 0; i < l; i++ {
		channels[i] = sender.syncRespPool.Get().(chan *protocol.SyncResponse)
	}
	// sync send parallel
	index := 0
	for _, sc := range sClients {
		go func(s *sClient) {
			channels[index] <- s.SyncSend(token, message)
		}(sc)
		index += 1
	}
	// get response and put
	resp = make([]*protocol.SyncResponse, l)
	for i := 0; i < l; i++ {
		resp[i] = <-channels[i]
		if resp[i].Err == nil {
			success += 1
		}
		sender.syncRespPool.Put(channels[i])
	}
	return
}

func (sender *sender) syncReceiveParallel(token, message []byte) {
	sClients := sender.ctx.syncer.Clients()
	l := len(sClients)
	if l == 0 {
		return
	}
	// must copy
	msg := make([]byte, len(message))
	copy(msg, message)
	// sync receive parallel
	for _, sc := range sClients {
		go func(s *sClient) {
			s.SyncReceive(token, msg)
		}(sc)
	}
}

// DeleteSyncSendM is used to delete syncSendM
// if delete role, must delete it
func (sender *sender) DeleteSyncSendM(role protocol.Role, guid string) {
	i := 0
	switch role {
	case protocol.Beacon:
		i = senderBeacon
	case protocol.Node:
		i = senderNode
	default:
		panic("invalid role")
	}
	sender.syncSendRWM[i].Lock()
	if _, ok := sender.syncSendMs[i][guid]; ok {
		delete(sender.syncSendMs[i], guid)
	}
	sender.syncSendRWM[i].Unlock()
}

// make sure send lock exist
func (sender *sender) lockRole(role protocol.Role, guid string) {
	i := 0
	switch role {
	case protocol.Beacon:
		i = senderBeacon
	case protocol.Node:
		i = senderNode
	}
	sender.syncSendRWM[i].Lock()
	if m, ok := sender.syncSendMs[i][guid]; ok {
		sender.syncSendRWM[i].Unlock()
		m.Lock()
	} else {
		sender.syncSendMs[i][guid] = new(sync.Mutex)
		sender.syncSendRWM[i].Unlock()
		sender.syncSendMs[i][guid].Lock()
	}
}

func (sender *sender) unlockRole(role protocol.Role, guid string) {
	i := 0
	switch role {
	case protocol.Beacon:
		i = senderBeacon
	case protocol.Node:
		i = senderNode
	}
	sender.syncSendRWM[i].RLock()
	if m, ok := sender.syncSendMs[i][guid]; ok {
		sender.syncSendRWM[i].RUnlock()
		m.Unlock()
	} else {
		sender.syncSendRWM[i].RUnlock()
	}
}

type senderWorker struct {
	ctx           *sender
	maxBufferSize int

	// task
	bt  *broadcastTask
	sst *syncSendTask
	srt *syncReceiveTask

	// key
	node   *mNode
	beacon *mBeacon
	aesKey []byte
	aesIV  []byte

	// prepare task objects
	preB  *protocol.Broadcast
	preSS *protocol.SyncSend
	preSR *protocol.SyncReceive

	guid           *guid.GUID
	buffer         *bytes.Buffer
	msgpackEncoder *msgpack.Encoder
	base64Encoder  io.WriteCloser
	hash           hash.Hash

	// temp
	nodeSyncer   *nodeSyncer
	beaconSyncer *beaconSyncer
	roleGUID     string
	token        []byte
	err          error
}

func (sw *senderWorker) handleSyncReceiveTask() {
	// check role
	if sw.srt.Role != protocol.Node && sw.srt.Role != protocol.Beacon {
		panic("sender.sender(): invalid srt.Role")
	}
	sw.preSR.GUID = sw.guid.Get()
	sw.preSR.Height = sw.srt.Height
	sw.preSR.Role = sw.srt.Role
	sw.preSR.RoleGUID = sw.srt.GUID
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preSR.GUID)
	sw.buffer.Write(convert.Uint64ToBytes(sw.preSR.Height))
	sw.buffer.WriteByte(sw.preSR.Role.Byte())
	sw.buffer.Write(sw.preSR.RoleGUID)
	sw.preSR.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack syncReceive & token
	sw.buffer.Reset()
	sw.err = sw.msgpackEncoder.Encode(sw.preSR)
	if sw.err != nil {
		panic(sw.err)
	}
	// send
	sw.token = append(protocol.Ctrl.Bytes(), sw.preSR.GUID...)
	sw.ctx.syncReceiveParallel(sw.token, sw.buffer.Bytes())
}

func (sw *senderWorker) handleSyncSendTask() {
	result := protocol.SyncResult{}
	// check role
	if sw.sst.Role != protocol.Node && sw.sst.Role != protocol.Beacon {
		if sw.sst.Result != nil {
			result.Err = protocol.ErrInvalidRole
			sw.sst.Result <- &result
		}
		return
	}
	sw.preSS.GUID = sw.guid.Get()
	// pack message(interface)
	if sw.sst.MessageI != nil {
		sw.buffer.Reset()
		sw.err = sw.msgpackEncoder.Encode(sw.sst.MessageI)
		if sw.err != nil {
			if sw.sst.Result != nil {
				result.Err = sw.err
				sw.sst.Result <- &result
			}
			return
		}
		sw.sst.Message = append(sw.sst.Command, sw.buffer.Bytes()...)
	}
	// set key
	switch sw.sst.Role {
	case protocol.Beacon:
		sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.sst.Target)
		if sw.err != nil {
			if sw.sst.Result != nil {
				result.Err = sw.err
				sw.sst.Result <- &result
			}
			return
		}
		sw.aesKey = sw.beacon.SessionKey
		sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	case protocol.Node:
		sw.node, sw.err = sw.ctx.ctx.db.SelectNode(sw.sst.Target)
		if sw.err != nil {
			if sw.sst.Result != nil {
				result.Err = sw.err
				sw.sst.Result <- &result
			}
			return
		}
		sw.aesKey = sw.node.SessionKey
		sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	default:
		panic("invalid sst.Role")
	}
	// hash
	sw.hash.Reset()
	sw.hash.Write(sw.sst.Message)
	sw.preSS.Hash = sw.hash.Sum(nil)
	// encrypt
	sw.preSS.Message, sw.err = aes.CBCEncrypt(sw.sst.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		if sw.sst.Result != nil {
			result.Err = sw.err
			sw.sst.Result <- &result
		}
		return
	}
	sw.preSS.ReceiverRole = sw.sst.Role
	sw.preSS.ReceiverGUID = sw.sst.Target
	// set sync height
	sw.buffer.Reset()
	_, _ = sw.base64Encoder.Write(sw.sst.Target)
	_ = sw.base64Encoder.Close()
	sw.roleGUID = sw.buffer.String()
	sw.ctx.lockRole(sw.sst.Role, sw.roleGUID)
	switch sw.sst.Role {
	case protocol.Beacon:
		sw.beaconSyncer, sw.err = sw.ctx.ctx.db.SelectBeaconSyncer(sw.sst.Target)
		if sw.err != nil {
			sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
			if sw.sst.Result != nil {
				result.Err = sw.err
				sw.sst.Result <- &result
			}
			return
		}
		sw.beaconSyncer.RLock()
		sw.preSS.Height = sw.beaconSyncer.CtrlSend
		sw.beaconSyncer.RUnlock()
	case protocol.Node:
		sw.nodeSyncer, sw.err = sw.ctx.ctx.db.SelectNodeSyncer(sw.sst.Target)
		if sw.err != nil {
			sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
			if sw.sst.Result != nil {
				result.Err = sw.err
				sw.sst.Result <- &result
			}
			return
		}
		sw.nodeSyncer.RLock()
		sw.preSS.Height = sw.nodeSyncer.CtrlSend
		sw.nodeSyncer.RUnlock()
	default:
		sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
		panic("invalid sst.Role")
	}
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preSS.GUID)
	sw.buffer.Write(convert.Uint64ToBytes(sw.preSS.Height))
	sw.buffer.Write(sw.preSS.Message)
	sw.buffer.Write(sw.preSS.Hash)
	sw.buffer.WriteByte(sw.preSS.SenderRole.Byte())
	sw.buffer.Write(sw.preSS.SenderGUID)
	sw.buffer.WriteByte(sw.preSS.ReceiverRole.Byte())
	sw.buffer.Write(sw.preSS.ReceiverGUID)
	sw.preSS.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack protocol.syncSend and token
	sw.buffer.Reset()
	sw.err = sw.msgpackEncoder.Encode(&sw.preSS)
	if sw.err != nil {
		sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
		if sw.sst.Result != nil {
			result.Err = sw.err
			sw.sst.Result <- &result
		}
		return
	}
	// !!! think order
	// first must add send height
	switch sw.sst.Role {
	case protocol.Beacon:
		sw.err = sw.ctx.ctx.db.UpdateBSCtrlSend(sw.sst.Target, sw.preSS.Height+1)
	case protocol.Node:
		sw.err = sw.ctx.ctx.db.UpdateNSCtrlSend(sw.sst.Target, sw.preSS.Height+1)
	default:
		sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
		panic("invalid sst.Role")
	}
	if sw.err != nil {
		sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
		if sw.sst.Result != nil {
			result.Err = sw.err
			sw.sst.Result <- &result
		}
		return
	}
	// !!! think order
	// second send
	sw.token = append(protocol.Ctrl.Bytes(), sw.preSS.GUID...)
	result.Response, result.Success =
		sw.ctx.syncSendParallel(sw.token, sw.buffer.Bytes())
	// !!! think order
	// rollback send height
	if result.Success == 0 {
		switch sw.sst.Role {
		case protocol.Beacon:
			sw.err = sw.ctx.ctx.db.UpdateBSCtrlSend(sw.sst.Target, sw.preSS.Height)
		case protocol.Node:
			sw.err = sw.ctx.ctx.db.UpdateNSCtrlSend(sw.sst.Target, sw.preSS.Height)
		default:
			sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
			panic("invalid sst.Role")
		}
		if sw.err != nil {
			sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
			if sw.sst.Result != nil {
				result.Err = sw.err
				sw.sst.Result <- &result
			}
			return
		}
	}
	sw.ctx.unlockRole(sw.sst.Role, sw.roleGUID)
	if sw.sst.Result != nil {
		sw.sst.Result <- &result
	}
}

func (sw *senderWorker) handleBroadcastTask() {
	result := protocol.BroadcastResult{}
	sw.preB.GUID = sw.guid.Get()
	// pack message
	if sw.bt.MessageI != nil {
		sw.buffer.Reset()
		sw.err = sw.msgpackEncoder.Encode(sw.bt.MessageI)
		if sw.err != nil {
			if sw.bt.Result != nil {
				result.Err = sw.err
				sw.bt.Result <- &result
			}
			return
		}
		sw.bt.Message = append(sw.bt.Command, sw.buffer.Bytes()...)
	}
	// hash
	sw.hash.Reset()
	sw.hash.Write(sw.bt.Message)
	sw.preB.Hash = sw.hash.Sum(nil)
	// encrypt
	sw.preB.Message, sw.err = sw.ctx.ctx.global.Encrypt(sw.bt.Message)
	if sw.err != nil {
		if sw.bt.Result != nil {
			result.Err = sw.err
			sw.bt.Result <- &result
		}
		return
	}
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preB.GUID)
	sw.buffer.Write(sw.preB.Message)
	sw.buffer.Write(sw.preB.Hash)
	sw.buffer.WriteByte(sw.preB.SenderRole.Byte())
	sw.buffer.Write(sw.preB.SenderGUID)
	sw.preB.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack broadcast & token
	sw.buffer.Reset()
	sw.err = sw.msgpackEncoder.Encode(sw.preB)
	if sw.err != nil {
		if sw.bt.Result != nil {
			result.Err = sw.err
			sw.bt.Result <- &result
		}
		return
	}
	// send
	sw.token = append(protocol.Ctrl.Bytes(), sw.preB.GUID...)
	result.Response, result.Success =
		sw.ctx.broadcastParallel(sw.token, sw.buffer.Bytes())
	if sw.bt.Result != nil {
		sw.bt.Result <- &result
	}
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
	sw.guid = guid.New(16, sw.ctx.ctx.global.Now)
	minBufferSize := guid.Size + 9
	sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
	sw.msgpackEncoder = msgpack.NewEncoder(sw.buffer)
	sw.base64Encoder = base64.NewEncoder(base64.StdEncoding, sw.buffer)
	sw.hash = sha256.New()
	// prepare task objects
	sw.preB = &protocol.Broadcast{
		SenderRole: protocol.Ctrl,
		SenderGUID: protocol.CtrlGUID,
	}
	sw.preSS = &protocol.SyncSend{
		SenderRole: protocol.Ctrl,
		SenderGUID: protocol.CtrlGUID,
	}
	sw.preSR = &protocol.SyncReceive{}
	// start handle task
	for {
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		case sw.srt = <-sw.ctx.syncReceiveTaskQueue:
			sw.handleSyncReceiveTask()
		case sw.sst = <-sw.ctx.syncSendTaskQueue:
			sw.handleSyncSendTask()
		case sw.bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask()
		case <-sw.ctx.stopSignal:
			return
		}
	}
}

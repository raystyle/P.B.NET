package node

import (
	"bytes"
	"crypto/sha256"
	"hash"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type broadcastTask struct {
	Command  []byte      // for Broadcast
	MessageI interface{} // for Broadcast
	Message  []byte      // for BroadcastPlugin
	Result   chan<- *protocol.BroadcastResult
}

type syncSendTask struct {
	Command  []byte      // for Send
	MessageI interface{} // for Send
	Message  []byte      // for SendPlugin
	Result   chan<- *protocol.SyncResult
}

type sender struct {
	ctx *NODE

	broadcastTaskQueue   chan *broadcastTask
	syncSendTaskQueue    chan *syncSendTask
	syncReceiveTaskQueue chan uint64 // height

	broadcastTaskPool sync.Pool
	syncSendTaskPool  sync.Pool

	broadcastDonePool sync.Pool
	syncSendDonePool  sync.Pool
	broadcastRespPool sync.Pool
	syncRespPool      sync.Pool

	syncSendM sync.Mutex // just send to Controller

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSender(ctx *NODE, cfg *Config) (*sender, error) {
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
		syncSendTaskQueue:    make(chan *syncSendTask, cfg.SenderQueueSize),
		syncReceiveTaskQueue: make(chan uint64, cfg.SenderQueueSize),
		stopSignal:           make(chan struct{}),
	}
	// init task sync pool
	sender.broadcastTaskPool.New = func() interface{} {
		return new(broadcastTask)
	}
	sender.syncSendTaskPool.New = func() interface{} {
		return new(syncSendTask)
	}
	// init done sync pool
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

func (sender *sender) Broadcast(
	command []byte,
	message interface{},
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	bt.Command = command
	bt.MessageI = message
	bt.Result = done
	sender.broadcastTaskQueue <- bt
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

func (sender *sender) BroadcastAsync(
	command []byte,
	message interface{},
	done chan<- *protocol.BroadcastResult,
) {
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	bt.Command = command
	bt.MessageI = message
	bt.Result = done
	sender.broadcastTaskQueue <- bt
}

func (sender *sender) BroadcastPlugin(
	message []byte,
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	bt.Message = message
	bt.Result = done
	sender.broadcastTaskQueue <- bt
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

func (sender *sender) Send(
	command []byte,
	message interface{},
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sst := sender.syncSendTaskPool.Get().(*syncSendTask)
	sst.Command = command
	sst.MessageI = message
	sst.Result = done
	sender.syncSendTaskQueue <- sst
	r = <-done
	sender.syncSendDonePool.Put(done)
	return
}

func (sender *sender) SendAsync(
	command []byte,
	message interface{},
	done chan<- *protocol.SyncResult,
) {
	sst := sender.syncSendTaskPool.Get().(*syncSendTask)
	sst.Command = command
	sst.MessageI = message
	sst.Result = done
	sender.syncSendTaskQueue <- sst
}

func (sender *sender) SendPlugin(
	message []byte,
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sst := sender.syncSendTaskPool.Get().(*syncSendTask)
	sst.Message = message
	sst.Result = done
	sender.syncSendTaskQueue <- sst
	r = <-done
	sender.syncSendDonePool.Put(done)
	return
}

// SyncReceive is used to sync node receive(controller send)
// notice node to delete message
// only for syncerWorker
func (sender *sender) SyncReceive(height uint64) {
	sender.syncReceiveTaskQueue <- height
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
	// if connect controller, first send
	ctrl := sender.ctx.syncer.CtrlConn()
	if ctrl != nil {
		br := ctrl.Broadcast(token, message)
		if br.Err == nil {
			success += 1
		}
	}

	/*
		sClients := sender.ctx.syncer.sClients()
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

	*/
	return
}

func (sender *sender) syncSendParallel(token, message []byte) (
	resp []*protocol.SyncResponse, success int) {
	// if connect controller, first send
	ctrl := sender.ctx.syncer.CtrlConn()
	if ctrl != nil {
		br := ctrl.SyncSend(token, message)
		if br.Err == nil {
			success += 1
		}
	}
	/*
		sClients := sender.ctx.syncer.sClients()
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

	*/
	return
}

func (sender *sender) syncReceiveParallel(token, message []byte) {
	// if connect controller, first send
	ctrl := sender.ctx.syncer.CtrlConn()
	if ctrl != nil {
		ctrl.SyncReceive(token, message)
	}
	/*
		sClients := sender.ctx.syncer.sClients()
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
	*/
}

type senderWorker struct {
	ctx           *sender
	maxBufferSize int

	// task
	bt  *broadcastTask
	sst *syncSendTask
	srt uint64 // height

	guid           *guid.GUID
	buffer         *bytes.Buffer
	msgpackEncoder *msgpack.Encoder
	hash           hash.Hash

	preB  *protocol.Broadcast
	preSS *protocol.SyncSend
	preSR *protocol.SyncReceive

	// temp
	token []byte
	err   error
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
	sw.guid = guid.New(16*(runtime.NumCPU()+1), sw.ctx.ctx.global.Now)
	// prepare buffer, msgpack encoder
	// syncReceiveTask = 1 + guid.Size + 8
	minBufferSize := guid.Size + 9
	sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
	sw.msgpackEncoder = msgpack.NewEncoder(sw.buffer)
	sw.hash = sha256.New()
	// prepare task objects
	sw.preB = &protocol.Broadcast{
		SenderRole: protocol.Node,
		SenderGUID: sw.ctx.ctx.global.GUID(),
	}
	sw.preSS = &protocol.SyncSend{
		SenderRole:   protocol.Node,
		SenderGUID:   sw.ctx.ctx.global.GUID(),
		ReceiverRole: protocol.Ctrl,
		ReceiverGUID: protocol.CtrlGUID,
	}
	sw.preSR = &protocol.SyncReceive{
		Role: protocol.Node,
		GUID: sw.ctx.ctx.global.GUID(),
	}
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

func (sw *senderWorker) handleSyncReceiveTask() {
	sw.preSR.GUID = sw.guid.Get()
	sw.preSR.Height = sw.srt
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
	sw.token = append(protocol.Node.Bytes(), sw.preSR.GUID...)
	sw.ctx.syncReceiveParallel(sw.token, sw.buffer.Bytes())
}

func (sw *senderWorker) handleSyncSendTask() {
	defer sw.ctx.syncSendTaskPool.Put(sw.sst)
	result := protocol.SyncResult{}
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
	// hash
	sw.hash.Reset()
	sw.hash.Write(sw.sst.Message)
	sw.preSS.Hash = sw.hash.Sum(nil)
	// encrypt
	sw.preSS.Message, sw.err = sw.ctx.ctx.global.Encrypt(sw.sst.Message)
	if sw.err != nil {
		if sw.sst.Result != nil {
			result.Err = sw.err
			sw.sst.Result <- &result
		}
		return
	}
	sw.ctx.syncSendM.Lock()
	defer sw.ctx.syncSendM.Unlock()
	// set sync height
	sw.preSS.Height = sw.ctx.ctx.global.GetSyncSendHeight()
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
	sw.err = sw.msgpackEncoder.Encode(sw.preSS)
	if sw.err != nil {
		if sw.sst.Result != nil {
			result.Err = sw.err
			sw.sst.Result <- &result
		}
		return
	}
	// add message to database (self)

	// !!! think order
	// first must add send height
	sw.ctx.ctx.global.SetSyncSendHeight(sw.preSS.Height + 1)
	// !!! think order
	// second send
	sw.token = append(protocol.Node.Bytes(), sw.preSS.GUID...)
	result.Response, result.Success = sw.ctx.syncSendParallel(sw.token, sw.buffer.Bytes())
	if sw.sst.Result != nil {
		sw.sst.Result <- &result
	}
}

func (sw *senderWorker) handleBroadcastTask() {
	defer sw.ctx.broadcastTaskPool.Put(sw.bt)
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
	sw.token = append(protocol.Node.Bytes(), sw.preB.GUID...)
	result.Response, result.Success = sw.ctx.broadcastParallel(sw.token, sw.buffer.Bytes())
	if sw.bt.Result != nil {
		sw.bt.Result <- &result
	}
}

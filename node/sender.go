package node

import (
	"bytes"
	"crypto/sha256"
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
	ctx              *NODE
	maxBufferSize    int
	broadcastQueue   chan *broadcastTask
	syncSendQueue    chan *syncSendTask
	syncReceiveQueue chan uint64 // height

	broadcastDonePool sync.Pool
	syncSendDonePool  sync.Pool
	broadcastRespPool sync.Pool
	syncRespPool      sync.Pool

	syncSendM sync.Mutex // just send to Controller

	guid       *guid.GUID
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSender(ctx *NODE, cfg *Config) (*sender, error) {
	// check config
	if cfg.SenderWorker < 1 {
		return nil, errors.New("sender number < 1")
	}
	if cfg.SenderQueueSize < 512 {
		return nil, errors.New("sender task queue size < 512")
	}
	sender := sender{
		ctx:              ctx,
		maxBufferSize:    cfg.MaxBufferSize,
		broadcastQueue:   make(chan *broadcastTask, cfg.SenderQueueSize),
		syncSendQueue:    make(chan *syncSendTask, cfg.SenderQueueSize),
		syncReceiveQueue: make(chan uint64, cfg.SenderQueueSize),
		guid:             guid.New(512*cfg.SenderWorker, ctx.global.Now),
		stopSignal:       make(chan struct{}),
	}
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
	// start senders
	for i := 0; i < cfg.SenderWorker; i++ {
		sender.wg.Add(1)
		go sender.worker()
	}
	return &sender, nil
}

func (sender *sender) Broadcast(
	command []byte,
	message interface{},
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	sender.broadcastQueue <- &broadcastTask{
		Command:  command,
		MessageI: message,
		Result:   done,
	}
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

func (sender *sender) BroadcastAsync(
	command []byte,
	message interface{},
	done chan<- *protocol.BroadcastResult,
) {
	sender.broadcastQueue <- &broadcastTask{
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

func (sender *sender) BroadcastPlugin(
	message []byte,
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	sender.broadcastQueue <- &broadcastTask{
		Message: message,
		Result:  done,
	}
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

func (sender *sender) Send(
	command []byte,
	message interface{},
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sender.syncSendQueue <- &syncSendTask{
		Command:  command,
		MessageI: message,
		Result:   done,
	}
	r = <-done
	sender.syncSendDonePool.Put(done)
	return
}

func (sender *sender) SendAsync(
	command []byte,
	message interface{},
	done chan<- *protocol.SyncResult,
) {
	sender.syncSendQueue <- &syncSendTask{
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

func (sender *sender) SendPlugin(
	message []byte,
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sender.syncSendQueue <- &syncSendTask{
		Message: message,
		Result:  done,
	}
	r = <-done
	sender.syncSendDonePool.Put(done)
	return
}

// SyncReceive is used to sync node receive(controller send)
// notice node to delete message
// only for syncer.worker()
func (sender *sender) SyncReceive(height uint64) {
	sender.syncReceiveQueue <- height
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

func (sender *sender) worker() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("sender.worker() panic:", r)
			sender.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			sender.wg.Add(1)
			go sender.worker()
		}
		sender.wg.Done()
	}()
	var (
		// task
		bt  *broadcastTask
		sst *syncSendTask
		srt uint64 // height

		// temp
		token []byte
		err   error
	)
	// prepare buffer, msgpack encoder
	// syncReceiveTask = 1 + guid.Size + 8
	minBufferSize := guid.Size + 9
	buffer := bytes.NewBuffer(make([]byte, minBufferSize))
	msgpackEncoder := msgpack.NewEncoder(buffer)
	hash := sha256.New()
	// prepare task objects
	preB := &protocol.Broadcast{
		SenderRole: protocol.Node,
		SenderGUID: sender.ctx.global.GUID(),
	}
	preSS := &protocol.SyncSend{
		SenderRole:   protocol.Node,
		SenderGUID:   sender.ctx.global.GUID(),
		ReceiverRole: protocol.Ctrl,
		ReceiverGUID: protocol.CtrlGUID,
	}
	preSR := &protocol.SyncReceive{
		Role: protocol.Node,
		GUID: sender.ctx.global.GUID(),
	}
	// start handle task
	for {
		// check buffer capacity
		if buffer.Cap() > sender.maxBufferSize {
			buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		// --------------------------sync receive-------------------------
		case srt = <-sender.syncReceiveQueue:
			preSR.GUID = sender.guid.Get()
			preSR.Height = srt
			// sign
			buffer.Reset()
			buffer.Write(preSR.GUID)
			buffer.Write(convert.Uint64ToBytes(preSR.Height))
			buffer.WriteByte(preSR.Role.Byte())
			buffer.Write(preSR.RoleGUID)
			preSR.Signature = sender.ctx.global.Sign(buffer.Bytes())
			// pack syncReceive & token
			buffer.Reset()
			err = msgpackEncoder.Encode(&preSR)
			if err != nil {
				panic(err)
			}
			// send
			token = append(protocol.Node.Bytes(), preSR.GUID...)
			sender.syncReceiveParallel(token, buffer.Bytes())
		// ---------------------------sync send---------------------------
		case sst = <-sender.syncSendQueue:
			result := protocol.SyncResult{}
			preSS.GUID = sender.guid.Get()
			// pack message(interface)
			if sst.MessageI != nil {
				buffer.Reset()
				err = msgpackEncoder.Encode(sst.MessageI)
				if err != nil {
					if sst.Result != nil {
						result.Err = err
						sst.Result <- &result
					}
					continue
				}
				sst.Message = append(sst.Command, buffer.Bytes()...)
			}
			// hash
			hash.Reset()
			hash.Write(sst.Message)
			preSS.Hash = hash.Sum(nil)
			// encrypt
			preSS.Message, err = sender.ctx.global.Encrypt(sst.Message)
			if err != nil {
				if sst.Result != nil {
					result.Err = err
					sst.Result <- &result
				}
				continue
			}
			sender.syncSendM.Lock()
			// set sync height
			preSS.Height = sender.ctx.global.GetSyncSendHeight()
			// sign
			buffer.Reset()
			buffer.Write(preSS.GUID)
			buffer.Write(convert.Uint64ToBytes(preSS.Height))
			buffer.Write(preSS.Message)
			buffer.Write(preSS.Hash)
			buffer.WriteByte(preSS.SenderRole.Byte())
			buffer.Write(preSS.SenderGUID)
			buffer.WriteByte(preSS.ReceiverRole.Byte())
			buffer.Write(preSS.ReceiverGUID)
			preSS.Signature = sender.ctx.global.Sign(buffer.Bytes())
			// pack protocol.syncSend and token
			buffer.Reset()
			err = msgpackEncoder.Encode(&preSS)
			if err != nil {
				sender.syncSendM.Unlock()
				if sst.Result != nil {
					result.Err = err
					sst.Result <- &result
				}
				continue
			}
			// !!! think order
			// first must add send height
			sender.ctx.global.SetSyncSendHeight(preSS.Height + 1)
			// !!! think order
			// second send
			token = append(protocol.Node.Bytes(), preSS.GUID...)
			result.Response, result.Success =
				sender.syncSendParallel(token, buffer.Bytes())
			// !!! think order
			// rollback send height
			if result.Success == 0 {
				sender.ctx.global.SetSyncSendHeight(preSS.Height)
			}
			sender.syncSendM.Unlock()
			if sst.Result != nil {
				sst.Result <- &result
			}
		// ---------------------------broadcast---------------------------
		case bt = <-sender.broadcastQueue:
			result := protocol.BroadcastResult{}
			preB.GUID = sender.guid.Get()
			// pack message
			if bt.MessageI != nil {
				buffer.Reset()
				err = msgpackEncoder.Encode(bt.MessageI)
				if err != nil {
					if bt.Result != nil {
						result.Err = err
						bt.Result <- &result
					}
					continue
				}
				bt.Message = append(bt.Command, buffer.Bytes()...)
			}
			// hash
			hash.Reset()
			hash.Write(bt.Message)
			preB.Hash = hash.Sum(nil)
			// encrypt
			preB.Message, err = sender.ctx.global.Encrypt(bt.Message)
			if err != nil {
				if bt.Result != nil {
					result.Err = err
					bt.Result <- &result
				}
				continue
			}
			// sign
			buffer.Reset()
			buffer.Write(preB.GUID)
			buffer.Write(preB.Message)
			buffer.Write(preB.Hash)
			buffer.WriteByte(preB.SenderRole.Byte())
			buffer.Write(preB.SenderGUID)
			preB.Signature = sender.ctx.global.Sign(buffer.Bytes())
			// pack broadcast & token
			buffer.Reset()
			err = msgpackEncoder.Encode(&preB)
			if err != nil {
				if bt.Result != nil {
					result.Err = err
					bt.Result <- &result
				}
				continue
			}
			// send
			token = append(protocol.Node.Bytes(), preB.GUID...)
			result.Response, result.Success =
				sender.broadcastParallel(token, buffer.Bytes())
			if bt.Result != nil {
				bt.Result <- &result
			}
		case <-sender.stopSignal:
			return
		}
	}
}

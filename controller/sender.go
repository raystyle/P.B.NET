package controller

import (
	"bytes"
	"sync"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
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
	Role     protocol.Role
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
	ctx               *CTRL
	maxBufferSize     int
	broadcastQueue    chan *broadcastTask
	syncSendQueue     chan *syncSendTask
	syncReceiveQueue  chan *syncReceiveTask
	broadcastDonePool sync.Pool
	syncSendDonePool  sync.Pool
	syncSendMs        [2]map[string]*sync.Mutex // role can be only one sync at th same time
	syncSendRWM       [2]sync.RWMutex           // key=base64(sender guid) 0=node 1=beacon
	guid              *guid.GUID
	stopSignal        chan struct{}
	wg                sync.WaitGroup
}

func newSender(ctx *CTRL, cfg *Config) (*sender, error) {
	// check config
	if cfg.SenderNumber < 1 {
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
		syncReceiveQueue: make(chan *syncReceiveTask, cfg.SenderQueueSize),
		guid:             guid.New(512*cfg.SenderNumber, ctx.global.Now),
		stopSignal:       make(chan struct{}),
	}
	sender.syncSendMs[senderNode] = make(map[string]*sync.Mutex)
	sender.syncSendMs[senderBeacon] = make(map[string]*sync.Mutex)
	// sync pool
	sender.broadcastDonePool.New = func() interface{} {
		return make(chan *protocol.BroadcastResult, 1)
	}
	sender.syncSendDonePool.New = func() interface{} {
		return make(chan *protocol.SyncResult, 1)
	}
	// start senders
	for i := 0; i < cfg.SenderNumber; i++ {
		sender.wg.Add(1)
		go sender.sender()
	}
	return &sender, nil
}

func (sender *sender) Broadcast(
	role protocol.Role,
	command []byte,
	message interface{},
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	sender.broadcastQueue <- &broadcastTask{
		Role:     role,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

func (sender *sender) BroadcastAsync(
	role protocol.Role,
	command []byte,
	message interface{},
	done chan<- *protocol.BroadcastResult,
) {
	sender.broadcastQueue <- &broadcastTask{
		Role:     role,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

func (sender *sender) BroadcastPlugin(
	role protocol.Role,
	message []byte,
) (r *protocol.BroadcastResult) {
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	sender.broadcastQueue <- &broadcastTask{
		Role:    role,
		Message: message,
		Result:  done,
	}
	r = <-done
	sender.broadcastDonePool.Put(done)
	return
}

func (sender *sender) Send(
	role protocol.Role,
	target,
	command []byte,
	message interface{},
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sender.syncSendQueue <- &syncSendTask{
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

func (sender *sender) SendAsync(
	role protocol.Role,
	target,
	command []byte,
	message interface{},
	done chan<- *protocol.SyncResult,
) {
	sender.syncSendQueue <- &syncSendTask{
		Role:     role,
		Target:   target,
		Command:  command,
		MessageI: message,
		Result:   done,
	}
}

func (sender *sender) SendPlugin(
	role protocol.Role,
	target,
	message []byte,
) (r *protocol.SyncResult) {
	done := sender.syncSendDonePool.Get().(chan *protocol.SyncResult)
	sender.syncSendQueue <- &syncSendTask{
		Role:    role,
		Target:  target,
		Message: message,
		Result:  done,
	}
	r = <-done
	sender.syncSendDonePool.Put(done)
	return
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

func (sender *sender) sender() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("sender panic:", r)
			sender.log(logger.FATAL, err)
			// start new sender
			sender.wg.Add(1)
			go sender.sender()
		}
		sender.wg.Done()
	}()
	var (
		// task
		bt  *broadcastTask
		sst *syncSendTask
		srt *syncReceiveTask

		// key
		// nodeKey   *database.Node_Key
		// beaconKey *database.Beacon_Key
		// aes_key    []byte
		// aes_iv     []byte

		// temp
		// roleGUID string
		// token []byte
		err error
	)
	// prepare buffer & msgpack encoder
	// syncReceiveTask = 1+ guid.SIZE + 8
	minBufferSize := guid.SIZE + 9
	buffer := bytes.NewBuffer(make([]byte, minBufferSize))
	encoder := msgpack.NewEncoder(buffer)
	// prepare task objects
	preB := &protocol.Broadcast{
		SenderRole: protocol.Ctrl,
		SenderGUID: protocol.CtrlGUID,
	}
	preSS := &protocol.SyncSend{
		SenderRole: protocol.Ctrl,
		SenderGUID: protocol.CtrlGUID,
	}
	preSR := &protocol.SyncReceive{}
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
			preSR.Height = srt.Height
			preSR.ReceiverRole = srt.Role
			preSR.ReceiverGUID = srt.GUID
			// sign
			buffer.Reset()
			buffer.Write(preSR.GUID)
			buffer.Write(convert.Uint64ToBytes(preSR.Height))
			buffer.WriteByte(preSR.ReceiverRole)
			buffer.Write(preSR.ReceiverGUID)
			preSR.Signature = sender.ctx.global.Sign(buffer.Bytes())
			// pack syncReceive & token
			buffer.Reset()
			err = encoder.Encode(&preSR)
			if err != nil {
				panic(err)
			}
			// token = append([]byte{protocol.Ctrl}, preSR.GUID...)
			// sync receive

		// ---------------------------sync send---------------------------
		case sst = <-sender.syncSendQueue:
			result := protocol.SyncResult{}
			// check role
			if sst.Role != protocol.Node && sst.Role != protocol.Beacon {
				if sst.Result != nil {
					result.Err = protocol.ErrInvalidRole
					sst.Result <- &result
				}
				continue
			}
			preSS.GUID = sender.guid.Get()
			// pack message(interface)
			if sst.MessageI != nil {
				buffer.Reset()
				err = encoder.Encode(sst.MessageI)
				if err != nil {
					if sst.Result != nil {
						result.Err = err
						sst.Result <- &result
					}
					continue
				}
				sst.Message = append(sst.Command, buffer.Bytes()...)
			}
			// set key

		// ---------------------------broadcast---------------------------
		case bt = <-sender.broadcastQueue:
			result := protocol.BroadcastResult{}
			preB.GUID = sender.guid.Get()
			// pack message
			if bt.MessageI != nil {
				buffer.Reset()
				err = encoder.Encode(bt.MessageI)
				if err != nil {
					if bt.Result != nil {
						result.Err = err
						bt.Result <- &result
					}
					continue
				}
				bt.Message = append(bt.Command, buffer.Bytes()...)
			}
			preB.Message, err = sender.ctx.global.Encrypt(bt.Message)
			if err != nil {
				if bt.Result != nil {
					result.Err = err
					bt.Result <- &result
				}
				continue
			}
			preB.ReceiverRole = bt.Role
			// sign
			buffer.Reset()
			buffer.Write(preB.GUID)
			buffer.Write(preB.Message)
			buffer.WriteByte(preB.SenderRole)
			buffer.Write(preB.SenderGUID)
			buffer.WriteByte(preB.ReceiverRole)
			preB.Signature = sender.ctx.global.Sign(buffer.Bytes())
			// pack broadcast & token
			buffer.Reset()
			err = encoder.Encode(&preB)
			if err != nil {
				if bt.Result != nil {
					result.Err = err
					bt.Result <- &result
				}
				continue
			}
			// token = append([]byte{protocol.Ctrl}, preB.GUID...)

		case <-sender.stopSignal:
			return
		}
	}
}

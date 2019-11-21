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

type sendTask struct {
	Command  []byte      // for Send
	MessageI interface{} // for Send
	Message  []byte      // for SendFromPlugin
	Result   chan<- *protocol.SendResult
}

// sender is used to send message to Controller
// it can connect other Node
type sender struct {
	ctx *Node

	sendTaskQueue chan *sendTask

	sendTaskPool     sync.Pool
	sendResultPool   sync.Pool
	sendDonePool     sync.Pool
	sendResponsePool sync.Pool

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSender(ctx *Node, config *Config) (*sender, error) {
	cfg := config.Sender

	// check config
	if cfg.Worker < 1 {
		return nil, errors.New("the number of the sender worker must >= 0")
	}
	if cfg.QueueSize < 128 {
		return nil, errors.New("sender task queue size must >= 128")
	}
	if cfg.MaxBufferSize < 512<<10 {
		return nil, errors.New("sender max buffer size must >= 512KB")
	}
	if cfg.Timeout < 15*time.Second {
		return nil, errors.New("sender timeout must >= 15s")
	}

	sender := sender{
		ctx:           ctx,
		sendTaskQueue: make(chan *sendTask, cfg.QueueSize),
		stopSignal:    make(chan struct{}),
	}

	// init sync pool
	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	sender.sendResponsePool.New = func() interface{} {
		return make(chan *protocol.SendResponse, 1)
	}

	// start sender workers
	sender.wg.Add(cfg.Worker)
	for i := 0; i < cfg.Worker; i++ {
		worker := senderWorker{
			ctx:           &sender,
			maxBufferSize: cfg.MaxBufferSize,
		}
		go worker.Work()
	}
	return &sender, nil
}

// SendAsync is used to asynchronous send message to Controller
// must put *protocol.Send to sender.sendResultPool
func (sender *sender) SendAsync(cmd []byte, msg interface{}, done chan<- *protocol.SendResult) {
	st := sender.sendTaskPool.Get().(*sendTask)
	st.Command = cmd
	st.MessageI = msg
	st.Result = done
	sender.sendTaskQueue <- st
}

// Send is used to send message to Controller
func (sender *sender) Send(cmd []byte, msg interface{}) error {
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	sender.SendAsync(cmd, msg, done)
	result := <-done
	defer func() {
		result.Clean()
		sender.sendResultPool.Put(result)
	}()
	err := result.Err
	if err != nil {
		sender.log(logger.Warning, "failed to send:", err)
		return err
	}
	return nil
}

// SendFromPlugin is used to provide a interface
// for plugins to send message to Controller
func (sender *sender) SendFromPlugin(message []byte) error {
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)

	st := sender.sendTaskPool.Get().(*sendTask)
	st.Message = message
	st.Result = done
	sender.sendTaskQueue <- st

	result := <-done
	defer func() {
		result.Clean()
		sender.sendResultPool.Put(result)
	}()
	err := result.Err
	if err != nil {
		sender.log(logger.Warning, "failed to send from plugin:", err)
		return err
	}
	return nil
}

func (sender *sender) Acknowledge(guid []byte) {

}

func (sender *sender) Close() {
	close(sender.stopSignal)
	sender.wg.Wait()
}

func (sender *sender) logf(l logger.Level, format string, log ...interface{}) {
	sender.ctx.logger.Printf(l, "sender", format, log...)
}

func (sender *sender) log(l logger.Level, log ...interface{}) {
	sender.ctx.logger.Print(l, "sender", log...)
}

type cSender interface {
	Send(token, message []byte) *protocol.SendResponse
}

// return responses and the number of the success
func (sender *sender) sendParallel(token, message []byte) ([]*protocol.SendResponse, int) {
	// clients := sender.ctx.syncer.Clients()

	sClients := sender.ctx.syncer.sClients()
	l := len(sClients)
	if l == 0 {
		return nil, 0
	}
	// padding channels
	channels := make([]chan *protocol.SyncResponse, l)
	for i := 0; i < l; i++ {
		channels[i] = sender.sendResponsePool.Get().(chan *protocol.SyncResponse)
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
		sender.sendResponsePool.Put(channels[i])
	}

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
	preSS *protocol.Send
	preSR *protocol.SyncReceive

	// temp
	token []byte
	err   error
}

func (sw *senderWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "sender.worker() panic:")
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
	sw.preSS = &protocol.Send{
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
		case sw.sst = <-sw.ctx.sendTaskQueue:
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
	defer sw.ctx.sendTaskPool.Put(sw.sst)

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

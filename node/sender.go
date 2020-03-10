package node

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/patch/msgpack"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xpanic"
)

// errors
var (
	ErrTooBigMessage  = fmt.Errorf("too big message")
	ErrNoConnections  = fmt.Errorf("no connections")
	ErrFailedToSend   = fmt.Errorf("failed to send")
	ErrFailedToAck    = fmt.Errorf("failed to acknowledge")
	ErrSendTimeout    = fmt.Errorf("send timeout")
	ErrSenderMaxConns = fmt.Errorf("sender with max connections")
	ErrSenderClosed   = fmt.Errorf("sender closed")
)

// sendTask is used to send message to the Controller.
// MessageI will be Encode by msgpack, except MessageI.(type) is []byte.
type sendTask struct {
	Ctx      context.Context
	Command  []byte      // for Send
	MessageI interface{} // for Send
	Message  []byte      // for SendFromPlugin
	Deflate  bool
	Result   chan<- *protocol.SendResult
}

// ackTask is used to acknowledge to the Controller.
type ackTask struct {
	SendGUID *guid.GUID
	Result   chan<- *protocol.AcknowledgeResult
}

// sender is used to send message to Controller, it can connect other Node.
type sender struct {
	ctx *Node

	sendTaskQueue chan *sendTask
	ackTaskQueue  chan *ackTask

	sendTaskPool sync.Pool
	ackTaskPool  sync.Pool

	sendDonePool sync.Pool
	ackDonePool  sync.Pool

	sendResultPool sync.Pool
	ackResultPool  sync.Pool

	deflateWriterPool sync.Pool

	// wait Controller acknowledge
	ackSlots    map[guid.GUID]chan struct{}
	ackSlotsRWM sync.RWMutex
	ackSlotPool sync.Pool

	guid *guid.Generator

	inClose int32
	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newSender(ctx *Node, config *Config) (*sender, error) {
	cfg := config.Sender

	// check config
	if cfg.Worker < 4 {
		return nil, errors.New("sender worker number must >= 4")
	}
	if cfg.Timeout < 15*time.Second {
		return nil, errors.New("sender timeout must >= 15 seconds")
	}
	if cfg.QueueSize < 512 {
		return nil, errors.New("sender queue size >= 512")
	}
	if cfg.MaxBufferSize < 16<<10 {
		return nil, errors.New("sender max buffer size must >= 16KB")
	}

	sender := &sender{
		ctx:           ctx,
		sendTaskQueue: make(chan *sendTask, cfg.QueueSize),
		ackTaskQueue:  make(chan *ackTask, cfg.QueueSize),
		ackSlots:      make(map[guid.GUID]chan struct{}),
	}
	sender.context, sender.cancel = context.WithCancel(context.Background())

	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.ackTaskPool.New = func() interface{} {
		return new(ackTask)
	}

	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	sender.ackDonePool.New = func() interface{} {
		return make(chan *protocol.AcknowledgeResult, 1)
	}

	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	sender.ackResultPool.New = func() interface{} {
		return new(protocol.AcknowledgeResult)
	}

	sender.deflateWriterPool.New = func() interface{} {
		writer, _ := flate.NewWriter(nil, flate.BestCompression)
		return writer
	}

	sender.ackSlotPool.New = func() interface{} {
		return make(chan struct{}, 1)
	}
	sender.guid = guid.New(cfg.QueueSize, ctx.global.Now)

	// start sender workers
	sender.wg.Add(2 * cfg.Worker)
	for i := 0; i < cfg.Worker; i++ {
		worker := senderWorker{
			ctx:           sender,
			timeout:       cfg.Timeout,
			maxBufferSize: cfg.MaxBufferSize,
			rand:          random.New(),
		}
		go worker.WorkWithBlock()
	}
	for i := 0; i < cfg.Worker; i++ {
		worker := senderWorker{
			ctx:           sender,
			maxBufferSize: cfg.MaxBufferSize,
			rand:          random.New(),
		}
		go worker.WorkWithoutBlock()
	}
	sender.wg.Add(1)
	go sender.ackSlotCleaner()
	return sender, nil
}

func (sender *sender) isClosed() bool {
	return atomic.LoadInt32(&sender.inClose) != 0
}

// func (sender *sender) logf(lv logger.Level, format string, log ...interface{}) {
// 	sender.ctx.logger.Printf(lv, "sender", format, log...)
// }

func (sender *sender) log(lv logger.Level, log ...interface{}) {
	sender.ctx.logger.Println(lv, "sender", log...)
}

// Synchronize is used to connect a node listener and start to synchronize
// can't connect if a exists client, or the target node is connected self.
func (sender *sender) Synchronize(
	ctx context.Context,
	guid *guid.GUID,
	listener *bootstrap.Listener,
) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	// check the number of clients
	current := len(sender.ctx.forwarder.GetClientConns())
	if current >= sender.ctx.forwarder.GetMaxClientConns() {
		return ErrSenderMaxConns
	}
	// check connect the exist connect node, include Node connected self
	for g := range sender.ctx.forwarder.GetNodeConns() {
		if g == *guid {
			const format = "this node already connected self\n%s"
			return errors.Errorf(format, guid.Hex())
		}
	}
	for g := range sender.ctx.forwarder.GetClientConns() {
		if g == *guid {
			const format = "already connected this node\n%s"
			return errors.Errorf(format, guid.Hex())
		}
	}
	// create client
	client, err := sender.ctx.NewClient(ctx, listener, guid)
	if err != nil {
		return errors.WithMessage(err, "failed to create client")
	}
	// interrupt
	wg := sync.WaitGroup{}
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(xpanic.Print(r, "sender.Synchronize"))
			}
			wg.Done()
		}()
		select {
		case <-done:
		case <-ctx.Done():
			client.Close()
		}
	}()
	defer func() {
		close(done)
		wg.Wait()
	}()
	// connect and start to synchronize
	var success bool
	defer func() {
		if !success {
			client.Close()
		}
	}()
	err = client.Connect()
	if err != nil {
		const format = "failed to connect node\nlistener: %s\n%s\nerror"
		return errors.WithMessagef(err, format, listener, guid.Hex())
	}
	err = client.Synchronize()
	if err != nil {
		const format = "failed to start to synchronize\nlistener: %s\n%s\nerror"
		return errors.WithMessagef(err, format, listener, guid)
	}
	success = true
	return nil
}

// Disconnect is used to disconnect Node.
func (sender *sender) Disconnect(guid *guid.GUID) error {
	if client, ok := sender.ctx.forwarder.GetClientConns()[*guid]; ok {
		client.Close()
		return nil
	}
	return errors.Errorf("client doesn't exist\n%s", guid)
}

// Send is used to send message to Controller.
func (sender *sender) Send(
	ctx context.Context,
	command []byte,
	message interface{},
	deflate bool,
) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Ctx = ctx
	st.Command = command
	st.MessageI = message
	st.Deflate = deflate
	st.Result = done
	// send to task queue
	select {
	case sender.sendTaskQueue <- st:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendFromPlugin is used to provide a interface for plugins to send message to Controller.
func (sender *sender) SendFromPlugin(message []byte, deflate bool) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Ctx = sender.context
	st.Message = message
	st.Deflate = deflate
	st.Result = done
	// send to task queue
	select {
	case sender.sendTaskQueue <- st:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// Acknowledge is used to acknowledge Controller that Node has received this message.
func (sender *sender) Acknowledge(send *protocol.Send) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.ackDonePool.Get().(chan *protocol.AcknowledgeResult)
	defer sender.ackDonePool.Put(done)
	at := sender.ackTaskPool.Get().(*ackTask)
	defer sender.ackTaskPool.Put(at)
	at.SendGUID = &send.GUID
	at.Result = done
	// send to task queue
	select {
	case sender.ackTaskQueue <- at:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.ackResultPool.Put(result)
	return result.Err
}

// HandleNodeAcknowledge is used to notice the Node that the Controller
// has received the send message.
func (sender *sender) HandleAcknowledge(send *guid.GUID) {
	sender.ackSlotsRWM.RLock()
	defer sender.ackSlotsRWM.RUnlock()
	ch := sender.ackSlots[*send]
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	case <-sender.context.Done():
	}
}

func (sender *sender) Close() {
	atomic.StoreInt32(&sender.inClose, 1)
	for {
		if len(sender.ackTaskQueue) == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	sender.cancel()
	sender.wg.Wait()
	sender.guid.Close()
	sender.ctx = nil
}

func (sender *sender) createAckSlot(send *guid.GUID) chan struct{} {
	ch := sender.ackSlotPool.Get().(chan struct{})
	sender.ackSlotsRWM.Lock()
	defer sender.ackSlotsRWM.Unlock()
	sender.ackSlots[*send] = ch
	return ch
}

func (sender *sender) destroyAckSlot(send *guid.GUID, ch chan struct{}) {
	sender.ackSlotsRWM.Lock()
	defer sender.ackSlotsRWM.Unlock()
	// when read channel timeout, worker call destroy(),
	// the channel maybe has signal, try to clean it.
	select {
	case <-ch:
	default:
	}
	sender.ackSlotPool.Put(ch)
	delete(sender.ackSlots, *send)
}

func (sender *sender) ackSlotCleaner() {
	defer func() {
		if r := recover(); r != nil {
			sender.log(logger.Fatal, xpanic.Print(r, "sender.ackSlotCleaner"))
			// restart slot cleaner
			time.Sleep(time.Second)
			go sender.ackSlotCleaner()
		} else {
			sender.wg.Done()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sender.cleanAckSlotMap()
		case <-sender.context.Done():
			return
		}
	}
}

func (sender *sender) cleanAckSlotMap() {
	sender.ackSlotsRWM.Lock()
	defer sender.ackSlotsRWM.Unlock()
	newMap := make(map[guid.GUID]chan struct{}, len(sender.ackSlots))
	for key, ch := range sender.ackSlots {
		newMap[key] = ch
	}
	sender.ackSlots = newMap
}

type senderWorker struct {
	ctx *sender

	timeout       time.Duration
	maxBufferSize int

	// runtime
	rand       *random.Rand
	buffer     *bytes.Buffer
	msgpack    *msgpack.Encoder
	deflateBuf *bytes.Buffer
	hash       hash.Hash

	// prepare task objects
	preS protocol.Send
	preA protocol.Acknowledge

	// shortcut
	forwarder *forwarder

	// receive acknowledge timeout
	timer *time.Timer
}

func (sw *senderWorker) WorkWithBlock() {
	defer func() {
		if r := recover(); r != nil {
			sw.ctx.log(logger.Fatal, xpanic.Print(r, "senderWorker.WorkWithBlock"))
			// restart worker
			time.Sleep(time.Second)
			go sw.WorkWithBlock()
		} else {
			sw.ctx.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.msgpack = msgpack.NewEncoder(sw.buffer)
	sw.deflateBuf = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.hash = sha256.New()
	sw.forwarder = sw.ctx.ctx.forwarder
	defer func() { sw.forwarder = nil }()
	// must stop at once, or maybe timeout at the first time.
	sw.timer = time.NewTimer(time.Minute)
	sw.timer.Stop()
	defer sw.timer.Stop()
	var (
		st *sendTask
		at *ackTask
	)
	for {
		select {
		case <-sw.ctx.context.Done():
			return
		default:
		}
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
		}
		select {
		case st = <-sw.ctx.sendTaskQueue:
			sw.handleSendTask(st)
		case at = <-sw.ctx.ackTaskQueue:
			sw.handleAcknowledgeTask(at)
		case <-sw.ctx.context.Done():
			return
		}
	}
}

func (sw *senderWorker) WorkWithoutBlock() {
	defer func() {
		if r := recover(); r != nil {
			sw.ctx.log(logger.Fatal, xpanic.Print(r, "senderWorker.WorkWithoutBlock"))
			// restart worker
			time.Sleep(time.Second)
			go sw.WorkWithoutBlock()
		} else {
			sw.ctx.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.forwarder = sw.ctx.ctx.forwarder
	defer func() { sw.forwarder = nil }()
	var at *ackTask
	for {
		select {
		case <-sw.ctx.context.Done():
			return
		default:
		}
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
		}
		select {
		case at = <-sw.ctx.ackTaskQueue:
			sw.handleAcknowledgeTask(at)
		case <-sw.ctx.context.Done():
			return
		}
	}
}

func (sw *senderWorker) handleSendTask(st *sendTask) {
	result := sw.ctx.sendResultPool.Get().(*protocol.SendResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleSendTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		st.Result <- result
	}()
	// pack
	sw.packSendData(st, result)
	if result.Err != nil {
		return
	}
	// send
	wait := sw.ctx.createAckSlot(&sw.preS.GUID)
	defer sw.ctx.destroyAckSlot(&sw.preS.GUID, wait)
	result.Responses, result.Success = sw.forwarder.Send(&sw.preS.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToSend
		return
	}
	// wait role acknowledge
	sw.timer.Reset(sw.timeout)
	select {
	case <-wait:
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
	case <-st.Ctx.Done():
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
		result.Err = st.Ctx.Err()
	case <-sw.timer.C:
		result.Err = ErrSendTimeout
	case <-sw.ctx.context.Done():
		result.Err = ErrSenderClosed
	}
}

func (sw *senderWorker) packSendData(st *sendTask, result *protocol.SendResult) {
	// pack message(interface)
	if st.MessageI != nil {
		sw.buffer.Reset()
		sw.buffer.Write(sw.rand.Bytes(messages.RandomDataSize))
		sw.buffer.Write(st.Command)
		if msg, ok := st.MessageI.([]byte); ok {
			sw.buffer.Write(msg)
		} else {
			result.Err = sw.msgpack.Encode(st.MessageI)
			if result.Err != nil {
				return
			}
		}
		// don't worry copy, because encrypt
		st.Message = sw.buffer.Bytes()
	}
	// hash
	sw.hash.Reset()
	sw.hash.Write(st.Message)
	sw.preS.Hash = sw.hash.Sum(nil)
	// compress message
	if st.Deflate {
		sw.preS.Deflate = 1
		writer := sw.ctx.deflateWriterPool.Get().(*flate.Writer)
		defer sw.ctx.deflateWriterPool.Put(writer)
		sw.deflateBuf.Reset()
		writer.Reset(sw.deflateBuf)
		_, result.Err = writer.Write(st.Message)
		if result.Err != nil {
			return
		}
		result.Err = writer.Close()
		if result.Err != nil {
			return
		}
		// check compressed message size
		if sw.deflateBuf.Len() > protocol.MaxFrameSize {
			result.Err = ErrTooBigMessage
			return
		}
		st.Message = sw.deflateBuf.Bytes()
	} else {
		sw.preS.Deflate = 0
	}
	// encrypt message
	sw.preS.Message, result.Err = sw.ctx.ctx.global.Encrypt(st.Message)
	if result.Err != nil {
		return
	}
	// set GUID
	sw.preS.GUID = *sw.ctx.guid.Get()
	sw.preS.RoleGUID = *sw.ctx.ctx.global.GUID()
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preS.GUID[:])
	sw.buffer.Write(sw.preS.RoleGUID[:])
	sw.buffer.Write(sw.preS.Hash)
	sw.buffer.WriteByte(sw.preS.Deflate)
	sw.buffer.Write(sw.preS.Message)
	sw.preS.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// self validate
	result.Err = sw.preS.Validate()
	if result.Err != nil {
		panic("sender handleSendTask error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preS.Pack(sw.buffer)
}

func (sw *senderWorker) handleAcknowledgeTask(at *ackTask) {
	result := sw.ctx.ackResultPool.Get().(*protocol.AcknowledgeResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleAcknowledgeTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		at.Result <- result
	}()
	sw.preA.GUID = *sw.ctx.guid.Get()
	sw.preA.RoleGUID = *sw.ctx.ctx.global.GUID()
	sw.preA.SendGUID = *at.SendGUID
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preA.GUID[:])
	sw.buffer.Write(sw.preA.RoleGUID[:])
	sw.buffer.Write(sw.preA.SendGUID[:])
	sw.preA.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// self validate
	result.Err = sw.preA.Validate()
	if result.Err != nil {
		panic("sender handleAcknowledgeTask error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preA.Pack(sw.buffer)
	// acknowledge
	result.Responses, result.Success = sw.forwarder.Acknowledge(&sw.preA.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToAck
	}
}

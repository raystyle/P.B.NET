package node

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

// errors
var (
	ErrNoConnections  = fmt.Errorf("no connections")
	ErrSendFailed     = fmt.Errorf("failed to send")
	ErrSendTimeout    = fmt.Errorf("send timeout")
	ErrSenderMaxConns = fmt.Errorf("sender with max connections")
	ErrSenderClosed   = fmt.Errorf("sender closed")
)

// MessageI will be Encode by msgpack, except MessageI.(type) is []byte
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
	ackTaskQueue  chan *guid.GUID

	sendTaskPool sync.Pool
	ackTaskPool  sync.Pool

	sendDonePool   sync.Pool
	sendResultPool sync.Pool

	// wait Controller acknowledge
	slots  map[guid.GUID]chan struct{}
	slotsM sync.Mutex

	guid *guid.Generator

	inClose    int32
	stopSignal chan struct{}
	wg         sync.WaitGroup
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
		ackTaskQueue:  make(chan *guid.GUID, cfg.QueueSize),
		slots:         make(map[guid.GUID]chan struct{}),
		stopSignal:    make(chan struct{}),
	}

	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.ackTaskPool.New = func() interface{} {
		return new(guid.GUID)
	}
	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	sender.guid = guid.New(cfg.QueueSize, ctx.global.Now)

	// start sender workers
	sender.wg.Add(cfg.Worker)
	for i := 0; i < cfg.Worker; i++ {
		worker := senderWorker{
			ctx:           sender,
			timeout:       cfg.Timeout,
			maxBufferSize: cfg.MaxBufferSize,
		}
		go worker.Work()
	}
	return sender, nil
}

func (sender *sender) isClosed() bool {
	return atomic.LoadInt32(&sender.inClose) != 0
}

// Synchronize is used to connect a node listener and start synchronize
// can't connect if a exists client, or the target node is connected self
func (sender *sender) Synchronize(ctx context.Context, guid *guid.GUID, bl *bootstrap.Listener) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	// check client number
	current := len(sender.ctx.forwarder.GetClientConns())
	if current >= sender.ctx.forwarder.GetMaxClientConns() {
		return ErrSenderMaxConns
	}
	// check connect the exist connect node(include the target node connect self)
	// these tag is the connection tag, not the node GUID
	for _, node := range sender.ctx.forwarder.GetNodeConns() {
		if *node.GUID == *guid {
			const format = "target node already connected self\n%s"
			return errors.Errorf(format, guid)
		}
	}
	for _, client := range sender.ctx.forwarder.GetClientConns() {
		if *client.GUID == *guid {
			const format = "already connected the target node\n%s"
			return errors.Errorf(format, guid)
		}
	}
	// connect
	const format = "failed to connect node\nlistener: %s\n%s"
	client, err := sender.ctx.NewClient(ctx, bl, guid)
	if err != nil {
		return errors.WithMessagef(err, format, bl, guid)
	}
	err = client.Connect()
	if err != nil {
		return errors.WithMessagef(err, format, bl, guid)
	}
	err = client.Synchronize()
	if err != nil {
		const format = "failed to start synchronize\nlistener: %s\n%s"
		return errors.WithMessagef(err, format, bl, guid)
	}
	const fStart = "start synchronize\nlistener: %s\n%s"
	sender.logf(logger.Info, fStart, bl, guid.Hex())
	return nil
}

// Send is used to send message to Controller
func (sender *sender) Send(cmd []byte, msg interface{}) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Command = cmd
	st.MessageI = msg
	st.Result = done
	// send to task queue
	select {
	case sender.sendTaskQueue <- st:
	case <-sender.stopSignal:
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendFromPlugin is used to provide a interface
// for plugins to send message to Controller
func (sender *sender) SendFromPlugin(message []byte) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Message = message
	st.Result = done
	// send to task queue
	select {
	case sender.sendTaskQueue <- st:
	case <-sender.stopSignal:
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// Acknowledge is used to acknowledge Controller that Node has received this message
func (sender *sender) Acknowledge(send *protocol.Send) {
	if sender.isClosed() {
		return
	}
	at := sender.ackTaskPool.Get().(*guid.GUID)
	*at = send.GUID
	select {
	case sender.ackTaskQueue <- at:
	case <-sender.stopSignal:
	}
}

// HandleNodeAcknowledge is used to notice the Node that the Controller
// has received the send message.
func (sender *sender) HandleAcknowledge(send *guid.GUID) {
	sender.slotsM.Lock()
	defer sender.slotsM.Unlock()
	ch := sender.slots[*send]
	if ch != nil {
		close(ch)
		delete(sender.slots, *send)
	}
}

func (sender *sender) createAckSlot(send *guid.GUID) (<-chan struct{}, func()) {
	sender.slotsM.Lock()
	defer sender.slotsM.Unlock()
	sender.slots[*send] = make(chan struct{})
	return sender.slots[*send], func() {
		sender.slotsM.Lock()
		defer sender.slotsM.Unlock()
		delete(sender.slots, *send)
	}
}

func (sender *sender) Close() {
	atomic.StoreInt32(&sender.inClose, 1)
	close(sender.stopSignal)
	sender.wg.Wait()
	sender.guid.Close()
	sender.ctx = nil
}

func (sender *sender) log(l logger.Level, log ...interface{}) {
	sender.ctx.logger.Println(l, "sender", log...)
}

func (sender *sender) logf(l logger.Level, format string, log ...interface{}) {
	sender.ctx.logger.Printf(l, "sender", format, log...)
}

type senderWorker struct {
	ctx *sender

	timeout       time.Duration
	maxBufferSize int

	// runtime
	buffer  *bytes.Buffer
	msgpack *msgpack.Encoder
	hash    hash.Hash
	err     error

	// prepare task objects
	preS protocol.Send
	preA protocol.Acknowledge

	// shortcut
	forwarder *forwarder

	// receive acknowledge timeout
	timer *time.Timer
}

func (sw *senderWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.Work")
			sw.ctx.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			go sw.Work()
		} else {
			sw.ctx.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.msgpack = msgpack.NewEncoder(sw.buffer)
	sw.hash = sha256.New()
	sw.forwarder = sw.ctx.ctx.forwarder
	sw.timer = time.NewTimer(sw.timeout)
	var (
		st *sendTask
		at *guid.GUID
	)
	for {
		select {
		case <-sw.ctx.stopSignal:
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
		case <-sw.ctx.stopSignal:
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
	// pack message(interface)
	if st.MessageI != nil {
		sw.buffer.Reset()
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
	// check message size
	if len(st.Message) > protocol.MaxFrameSize {
		result.Err = protocol.ErrTooBigFrame
		return
	}
	// encrypt
	sw.preS.Message, result.Err = sw.ctx.ctx.global.Encrypt(st.Message)
	if result.Err != nil {
		return
	}
	// set GUID
	sw.preS.GUID = *sw.ctx.guid.Get()
	sw.preS.RoleGUID = *sw.ctx.ctx.global.GUID()
	// hash
	sw.hash.Reset()
	sw.hash.Write(st.Message)
	sw.preS.Hash = sw.hash.Sum(nil)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preS.GUID[:])
	sw.buffer.Write(sw.preS.RoleGUID[:])
	sw.buffer.Write(sw.preS.Hash)
	sw.buffer.Write(sw.preS.Message)
	sw.preS.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// self validate
	sw.err = sw.preS.Validate()
	if sw.err != nil {
		panic("sender internal error: " + sw.err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preS.Pack(sw.buffer)

	// send
	wait, destroy := sw.ctx.createAckSlot(&sw.preS.GUID)
	result.Responses, result.Success = sw.forwarder.Send(&sw.preS.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrSendFailed
		return
	}
	// wait role acknowledge
	if !sw.timer.Stop() {
		<-sw.timer.C
	}
	sw.timer.Reset(sw.timeout)
	select {
	case <-wait:
	case <-sw.timer.C:
		destroy()
		result.Err = ErrSendTimeout
	case <-sw.ctx.stopSignal:
		result.Err = ErrSenderClosed
	}
}

func (sw *senderWorker) handleAcknowledgeTask(at *guid.GUID) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleAcknowledgeTask")
			sw.ctx.log(logger.Fatal, err)
		}
		sw.ctx.ackTaskPool.Put(at)
	}()
	sw.preA.GUID = *sw.ctx.guid.Get()
	sw.preA.RoleGUID = *sw.ctx.ctx.global.GUID()
	sw.preA.SendGUID = *at
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preA.GUID[:])
	sw.buffer.Write(sw.preA.RoleGUID[:])
	sw.buffer.Write(sw.preA.SendGUID[:])
	sw.preA.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// self validate
	sw.err = sw.preA.Validate()
	if sw.err != nil {
		panic("sender internal error: " + sw.err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preA.Pack(sw.buffer)
	sw.forwarder.Acknowledge(&sw.preA.GUID, sw.buffer)
}

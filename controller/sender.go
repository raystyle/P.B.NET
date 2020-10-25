package controller

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/aes"
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
	ErrTooLargeMessage      = fmt.Errorf("too large message")
	ErrNoConnections        = fmt.Errorf("sender is not connected to any nodes")
	ErrFailedToSendToNode   = fmt.Errorf("failed to send to node")
	ErrFailedToSendToBeacon = fmt.Errorf("failed to send to beacon")
	ErrFailedToAckToNode    = fmt.Errorf("failed to acknowledge to node")
	ErrFailedToAckToBeacon  = fmt.Errorf("failed to acknowledge to beacon")
	ErrFailedToBroadcast    = fmt.Errorf("failed to broadcast")
	ErrFailedToAnswer       = fmt.Errorf("failed to answer")
	ErrSendTimeout          = fmt.Errorf("send timeout")
	ErrSenderMaxConns       = fmt.Errorf("sender with max connections")
	ErrSenderClosed         = fmt.Errorf("sender closed")
)

// broadcastTask is used to broadcast message to all Nodes
// MessageI will be Encode by msgpack, except MessageI.(type) is []byte
type broadcastTask struct {
	Command  []byte      // for Broadcast
	MessageI interface{} // for Broadcast
	Message  []byte      // for BroadcastFromPlugin
	Deflate  bool
	Result   chan<- *protocol.BroadcastResult
}

// sendTask is used to send message to the target Node or Beacon.
// MessageI will be Encode by msgpack, except MessageI.(type) is []byte.
type sendTask struct {
	ctx context.Context

	GUID     *guid.GUID  // receiver role's GUID
	Command  []byte      // for Send
	MessageI interface{} // for Send
	Message  []byte      // for SendFromPlugin
	Deflate  bool
	Result   chan<- *protocol.SendResult
}

// ackTask is used to acknowledge to the Node or Beacon.
type ackTask struct {
	RoleGUID *guid.GUID
	SendGUID *guid.GUID
	Result   chan<- *protocol.AcknowledgeResult
}

// answerTask is used to answer the Beacon queried message.
type answerTask struct {
	BeaconGUID *guid.GUID
	Index      uint64
	Deflate    byte
	Message    []byte
	Result     chan<- *protocol.AnswerResult
}

// wait role acknowledge
type roleAckSlot struct {
	// key = Send.GUID
	slots map[guid.GUID]chan struct{}
	rwm   sync.RWMutex
}

type sender struct {
	ctx *Ctrl

	maxConns atomic.Value

	sendToNodeTaskQueue   chan *sendTask
	sendToBeaconTaskQueue chan *sendTask
	ackToNodeTaskQueue    chan *ackTask
	ackToBeaconTaskQueue  chan *ackTask
	broadcastTaskQueue    chan *broadcastTask
	answerTaskQueue       chan *answerTask

	sendTaskPool      sync.Pool
	ackTaskPool       sync.Pool
	broadcastTaskPool sync.Pool
	answerTaskPool    sync.Pool

	sendDonePool      sync.Pool
	ackDonePool       sync.Pool
	broadcastDonePool sync.Pool
	answerDonePool    sync.Pool

	sendResultPool      sync.Pool
	ackResultPool       sync.Pool
	broadcastResultPool sync.Pool
	answerResultPool    sync.Pool

	deflateWriterPool sync.Pool

	// key = Node GUID
	clients    map[guid.GUID]*Client
	clientsRWM sync.RWMutex

	// check beacon is in interactive mode
	interactive    map[guid.GUID]bool
	interactiveRWM sync.RWMutex

	// receive acknowledge, key = role GUID
	nodeAckSlots      map[guid.GUID]*roleAckSlot
	nodeAckSlotsRWM   sync.RWMutex
	beaconAckSlots    map[guid.GUID]*roleAckSlot
	beaconAckSlotsRWM sync.RWMutex
	ackSlotPool       sync.Pool

	guid *guid.Generator

	inClose int32
	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newSender(ctx *Ctrl, config *Config) (*sender, error) {
	cfg := config.Sender

	// check config
	if cfg.MaxConns < 1 {
		return nil, errors.New("sender max conns >= 1")
	}
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
		ctx:                   ctx,
		sendToNodeTaskQueue:   make(chan *sendTask, cfg.QueueSize),
		sendToBeaconTaskQueue: make(chan *sendTask, cfg.QueueSize),
		ackToNodeTaskQueue:    make(chan *ackTask, cfg.QueueSize),
		ackToBeaconTaskQueue:  make(chan *ackTask, cfg.QueueSize),
		broadcastTaskQueue:    make(chan *broadcastTask, cfg.QueueSize),
		answerTaskQueue:       make(chan *answerTask, cfg.QueueSize),
		clients:               make(map[guid.GUID]*Client, cfg.MaxConns),
		interactive:           make(map[guid.GUID]bool),
		nodeAckSlots:          make(map[guid.GUID]*roleAckSlot),
		beaconAckSlots:        make(map[guid.GUID]*roleAckSlot),
	}
	sender.context, sender.cancel = context.WithCancel(context.Background())

	maxConns := cfg.MaxConns
	sender.maxConns.Store(maxConns)

	// initialize sync pool
	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.ackTaskPool.New = func() interface{} {
		return new(ackTask)
	}
	sender.broadcastTaskPool.New = func() interface{} {
		return new(broadcastTask)
	}
	sender.answerTaskPool.New = func() interface{} {
		return &answerTask{BeaconGUID: new(guid.GUID)}
	}

	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	sender.ackDonePool.New = func() interface{} {
		return make(chan *protocol.AcknowledgeResult, 1)
	}
	sender.broadcastDonePool.New = func() interface{} {
		return make(chan *protocol.BroadcastResult, 1)
	}
	sender.answerDonePool.New = func() interface{} {
		return make(chan *protocol.AnswerResult, 1)
	}

	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	sender.ackResultPool.New = func() interface{} {
		return new(protocol.AcknowledgeResult)
	}
	sender.broadcastResultPool.New = func() interface{} {
		return new(protocol.BroadcastResult)
	}
	sender.answerResultPool.New = func() interface{} {
		return new(protocol.AnswerResult)
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
			rand:          random.NewRand(),
		}
		go worker.WorkWithBlock()
	}
	for i := 0; i < cfg.Worker; i++ {
		worker := senderWorker{
			ctx:           sender,
			maxBufferSize: cfg.MaxBufferSize,
			rand:          random.NewRand(),
		}
		go worker.WorkWithoutBlock()
	}
	sender.wg.Add(1)
	go sender.ackSlotCleaner()
	return sender, nil
}

// GetMaxConns is used to get sender max connection.
func (sender *sender) GetMaxConns() int {
	return sender.maxConns.Load().(int)
}

// SetMaxConns is used to set sender max connection.
func (sender *sender) SetMaxConns(n int) error {
	if n < 1 {
		return errors.New("max conns must >= 1")
	}
	sender.maxConns.Store(n)
	return nil
}

// Clients is used to get all clients that start Synchronize
func (sender *sender) Clients() map[guid.GUID]*Client {
	sender.clientsRWM.RLock()
	defer sender.clientsRWM.RUnlock()
	clients := make(map[guid.GUID]*Client, len(sender.clients))
	for key, client := range sender.clients {
		clients[key] = client
	}
	return clients
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

func (sender *sender) checkNode(guid *guid.GUID) error {
	sender.clientsRWM.RLock()
	defer sender.clientsRWM.RUnlock()
	if len(sender.clients) >= sender.GetMaxConns() {
		return ErrSenderMaxConns
	}
	if _, ok := sender.clients[*guid]; ok {
		return errors.Errorf("already connected this node\n%s", guid.Hex())
	}
	return nil
}

// Synchronize is used to connect a node listener and start to synchronize.
func (sender *sender) Synchronize(
	ctx context.Context,
	guid *guid.GUID,
	listener *bootstrap.Listener,
) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	// must simple check firstly
	err := sender.checkNode(guid)
	if err != nil {
		return err
	}
	// create client
	client, err := sender.ctx.NewClient(ctx, listener, guid, func() {
		sender.clientsRWM.Lock()
		defer sender.clientsRWM.Unlock()
		delete(sender.clients, *guid)
	})
	if err != nil {
		return err
	}
	// interrupt
	wg := sync.WaitGroup{}
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				sender.log(logger.Fatal, xpanic.Print(r, "sender.Synchronize"))
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
	var ok bool
	defer func() {
		if !ok {
			client.Close()
		}
	}()
	err = client.Synchronize()
	if err != nil {
		const format = "failed to start to synchronize\nlistener: %s\n%s\nerror"
		return errors.WithMessagef(err, format, listener, guid.Hex())
	}
	// must check twice
	sender.clientsRWM.Lock()
	defer sender.clientsRWM.Unlock()
	if len(sender.clients) >= sender.GetMaxConns() {
		return ErrSenderMaxConns
	}
	if _, ok := sender.clients[*guid]; ok {
		return errors.Errorf("already connected this node\n%s", guid.Hex())
	}
	sender.clients[*guid] = client
	ok = true
	return nil
}

// Disconnect is used to disconnect Node.
func (sender *sender) Disconnect(guid *guid.GUID) error {
	if client, ok := sender.Clients()[*guid]; ok {
		client.Close()
		return nil
	}
	return errors.Errorf("client is not exist\n%s", guid)
}

// SendToNode is used to send message to the target Node,
// it will block until the target Node acknowledge.
// You can cancel context to interrupt wait acknowledge.
// Cancel context is useless until send messages to all clients(it fast).
func (sender *sender) SendToNode(
	ctx context.Context,
	guid *guid.GUID,
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
	st.ctx = ctx
	st.GUID = guid
	st.Command = command
	st.MessageI = message
	st.Deflate = deflate
	st.Result = done
	// send to task queue
	select {
	case sender.sendToNodeTaskQueue <- st:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendToBeacon is used to send message to the target Beacon,
// it will block until the target Beacon acknowledge.
// You can cancel context to interrupt wait acknowledge.
// Cancel context is useless until send messages to all clients(it fast).
func (sender *sender) SendToBeacon(
	ctx context.Context,
	guid *guid.GUID,
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
	st.ctx = ctx
	st.GUID = guid
	st.Command = command
	st.MessageI = message
	st.Deflate = deflate
	st.Result = done
	// send to task queue
	select {
	case sender.sendToBeaconTaskQueue <- st:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendToNodeFromPlugin is used to send message to the target Node from plugin.
func (sender *sender) SendToNodeFromPlugin(GUID, message []byte, deflate bool) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	g := new(guid.GUID)
	err := g.Write(GUID)
	if err != nil {
		return err
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.ctx = sender.context
	st.GUID = g
	st.Message = message
	st.Deflate = deflate
	st.Result = done
	// send to task queue
	select {
	case sender.sendToNodeTaskQueue <- st:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendToBeaconFromPlugin is used to send message to the target Beacon from plugin.
func (sender *sender) SendToBeaconFromPlugin(GUID, message []byte, deflate bool) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	g := new(guid.GUID)
	err := g.Write(GUID)
	if err != nil {
		return err
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.ctx = sender.context
	st.GUID = g
	st.Message = message
	st.Deflate = deflate
	st.Result = done
	// send to task queue
	select {
	case sender.sendToBeaconTaskQueue <- st:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// AckToNode is used to acknowledge Node that Controller has received this message.
func (sender *sender) AckToNode(send *protocol.Send) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.ackDonePool.Get().(chan *protocol.AcknowledgeResult)
	defer sender.ackDonePool.Put(done)
	at := sender.ackTaskPool.Get().(*ackTask)
	defer sender.ackTaskPool.Put(at)
	at.RoleGUID = &send.RoleGUID
	at.SendGUID = &send.GUID
	at.Result = done
	// send to task queue
	select {
	case sender.ackToNodeTaskQueue <- at:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.ackResultPool.Put(result)
	return result.Err
}

// AckToBeacon is used to acknowledge Beacon that Controller has received this message.
func (sender *sender) AckToBeacon(send *protocol.Send) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.ackDonePool.Get().(chan *protocol.AcknowledgeResult)
	defer sender.ackDonePool.Put(done)
	at := sender.ackTaskPool.Get().(*ackTask)
	defer sender.ackTaskPool.Put(at)
	at.RoleGUID = &send.RoleGUID
	at.SendGUID = &send.GUID
	at.Result = done
	// send to task queue
	select {
	case sender.ackToBeaconTaskQueue <- at:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.ackResultPool.Put(result)
	return result.Err
}

// Broadcast is used to broadcast message to all Nodes.
func (sender *sender) Broadcast(command []byte, message interface{}, deflate bool) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	defer sender.broadcastDonePool.Put(done)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	defer sender.broadcastTaskPool.Put(bt)
	bt.Command = command
	bt.MessageI = message
	bt.Deflate = deflate
	bt.Result = done
	// send to task queue
	select {
	case sender.broadcastTaskQueue <- bt:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.broadcastResultPool.Put(result)
	return result.Err
}

// BroadcastFromPlugin is used to broadcast message to all Nodes from plugin
func (sender *sender) BroadcastFromPlugin(message []byte, deflate bool) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	defer sender.broadcastDonePool.Put(done)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	defer sender.broadcastTaskPool.Put(bt)
	bt.Message = message
	bt.Deflate = deflate
	bt.Result = done
	// send to task queue
	select {
	case sender.broadcastTaskQueue <- bt:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.broadcastResultPool.Put(result)
	return result.Err
}

func (sender *sender) getNodeAckSlot(role *guid.GUID) *roleAckSlot {
	sender.nodeAckSlotsRWM.RLock()
	defer sender.nodeAckSlotsRWM.RUnlock()
	return sender.nodeAckSlots[*role]
}

// HandleNodeAcknowledge is used to notice the Controller that the
// target Node has received the send message.
func (sender *sender) HandleNodeAcknowledge(role, send *guid.GUID) {
	nas := sender.getNodeAckSlot(role)
	if nas == nil {
		return
	}
	nas.rwm.RLock()
	defer nas.rwm.RUnlock()
	if ch, ok := nas.slots[*send]; ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (sender *sender) getBeaconAckSlot(role *guid.GUID) *roleAckSlot {
	sender.beaconAckSlotsRWM.RLock()
	defer sender.beaconAckSlotsRWM.RUnlock()
	return sender.beaconAckSlots[*role]
}

// HandleNodeAcknowledge is used to notice the Controller that the
// target Beacon has received the send message.
func (sender *sender) HandleBeaconAcknowledge(role, send *guid.GUID) {
	bas := sender.getBeaconAckSlot(role)
	if bas == nil {
		return
	}
	bas.rwm.RLock()
	defer bas.rwm.RUnlock()
	if ch, ok := bas.slots[*send]; ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Answer is used to answer Beacon query message.
func (sender *sender) Answer(msg *mBeaconMessage) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.answerDonePool.Get().(chan *protocol.AnswerResult)
	defer sender.answerDonePool.Put(done)
	rt := sender.answerTaskPool.Get().(*answerTask)
	defer sender.answerTaskPool.Put(rt)
	err := rt.BeaconGUID.Write(msg.GUID)
	if err != nil {
		panic("sender Answer error: " + err.Error())
	}
	rt.Index = msg.Index
	rt.Deflate = msg.Deflate
	rt.Message = msg.Message
	rt.Result = done
	// send to task queue
	select {
	case sender.answerTaskQueue <- rt:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.answerResultPool.Put(result)
	return result.Err
}

func (sender *sender) DeleteNodeAckSlots(guid *guid.GUID) {
	sender.nodeAckSlotsRWM.Lock()
	defer sender.nodeAckSlotsRWM.Unlock()
	delete(sender.nodeAckSlots, *guid)
}

func (sender *sender) DeleteBeaconAckSlots(guid *guid.GUID) {
	sender.beaconAckSlotsRWM.Lock()
	defer sender.beaconAckSlotsRWM.Unlock()
	delete(sender.beaconAckSlots, *guid)
}

func (sender *sender) EnableInteractiveMode(guid *guid.GUID) {
	sender.interactiveRWM.Lock()
	defer sender.interactiveRWM.Unlock()
	sender.interactive[*guid] = true
}

func (sender *sender) DisableInteractiveMode(guid *guid.GUID) {
	sender.interactiveRWM.Lock()
	defer sender.interactiveRWM.Unlock()
	delete(sender.interactive, *guid)
}

func (sender *sender) IsInInteractiveMode(guid *guid.GUID) bool {
	sender.interactiveRWM.RLock()
	defer sender.interactiveRWM.RUnlock()
	return sender.interactive[*guid]
}

func (sender *sender) Close() {
	atomic.StoreInt32(&sender.inClose, 1)
	for {
		if len(sender.ackToNodeTaskQueue) == 0 &&
			len(sender.ackToBeaconTaskQueue) == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	sender.cancel()
	sender.wg.Wait() // wait all acknowledge task finish
	for {
		// disconnect all sender client
		for _, client := range sender.Clients() {
			client.Close()
		}
		// wait close
		time.Sleep(10 * time.Millisecond)
		if len(sender.Clients()) == 0 {
			break
		}
	}
	sender.guid.Close()
	sender.ctx = nil
}

func (sender *sender) mustGetNodeAckSlot(role *guid.GUID) *roleAckSlot {
	sender.nodeAckSlotsRWM.Lock()
	defer sender.nodeAckSlotsRWM.Unlock()
	nas := sender.nodeAckSlots[*role]
	if nas != nil {
		return nas
	}
	ras := &roleAckSlot{
		slots: make(map[guid.GUID]chan struct{}),
	}
	sender.nodeAckSlots[*role] = ras
	return ras
}

func (sender *sender) createNodeAckSlot(role, send *guid.GUID) chan struct{} {
	ch := sender.ackSlotPool.Get().(chan struct{})
	nas := sender.mustGetNodeAckSlot(role)
	nas.rwm.Lock()
	defer nas.rwm.Unlock()
	nas.slots[*send] = ch
	return ch
}

func (sender *sender) destroyNodeAckSlot(role, send *guid.GUID, ch chan struct{}) {
	nas := sender.mustGetNodeAckSlot(role)
	nas.rwm.Lock()
	defer nas.rwm.Unlock()
	// when read channel timeout, worker call destroy(),
	// the channel maybe has signal, try to clean it.
	select {
	case <-ch:
	default:
	}
	sender.ackSlotPool.Put(ch)
	delete(nas.slots, *send)
}

func (sender *sender) mustGetBeaconAckSlot(role *guid.GUID) *roleAckSlot {
	sender.beaconAckSlotsRWM.Lock()
	defer sender.beaconAckSlotsRWM.Unlock()
	bas := sender.beaconAckSlots[*role]
	if bas != nil {
		return bas
	}
	ras := &roleAckSlot{
		slots: make(map[guid.GUID]chan struct{}),
	}
	sender.beaconAckSlots[*role] = ras
	return ras
}

func (sender *sender) createBeaconAckSlot(role, send *guid.GUID) chan struct{} {
	ch := sender.ackSlotPool.Get().(chan struct{})
	bas := sender.mustGetBeaconAckSlot(role)
	bas.rwm.Lock()
	defer bas.rwm.Unlock()
	bas.slots[*send] = ch
	return ch
}

func (sender *sender) destroyBeaconAckSlot(role, send *guid.GUID, ch chan struct{}) {
	bas := sender.mustGetBeaconAckSlot(role)
	bas.rwm.Lock()
	defer bas.rwm.Unlock()
	// when read channel timeout, worker call destroy(),
	// the channel maybe has signal, try to clean it.
	select {
	case <-ch:
	default:
	}
	sender.ackSlotPool.Put(ch)
	delete(bas.slots, *send)
}

func (sender *sender) sendToNode(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.SendResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// send parallel
	response := make(chan *protocol.SendResponse)
	for _, client := range clients {
		go func(client *Client) {
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Print(r, "sender.sendToNode")
					sender.log(logger.Fatal, buf)
				}
			}()
			response <- client.SendToNode(guid, data)
		}(client)
	}
	var success int
	responses := make([]*protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
}

func (sender *sender) sendToBeacon(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.SendResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// send parallel
	response := make(chan *protocol.SendResponse)
	for _, client := range clients {
		go func(client *Client) {
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Print(r, "sender.sendToBeacon")
					sender.log(logger.Fatal, buf)
				}
			}()
			response <- client.SendToBeacon(guid, data)
		}(client)
	}
	var success int
	responses := make([]*protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
}

func (sender *sender) ackToNode(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.AcknowledgeResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// acknowledge parallel
	response := make(chan *protocol.AcknowledgeResponse, l)
	for _, client := range clients {
		go func(client *Client) {
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Print(r, "sender.ackToNode")
					sender.log(logger.Fatal, buf)
				}
			}()
			response <- client.AckToNode(guid, data)
		}(client)
	}
	var success int
	responses := make([]*protocol.AcknowledgeResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
}

func (sender *sender) ackToBeacon(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.AcknowledgeResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// acknowledge parallel
	response := make(chan *protocol.AcknowledgeResponse, l)
	for _, client := range clients {
		go func(client *Client) {
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Print(r, "sender.ackToBeacon")
					sender.log(logger.Fatal, buf)
				}
			}()
			response <- client.AckToBeacon(guid, data)
		}(client)
	}
	var success int
	responses := make([]*protocol.AcknowledgeResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
}

func (sender *sender) broadcast(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.BroadcastResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// broadcast parallel
	response := make(chan *protocol.BroadcastResponse)
	for _, client := range clients {
		go func(client *Client) {
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Print(r, "sender.broadcast")
					sender.log(logger.Fatal, buf)
				}
			}()
			response <- client.Broadcast(guid, data)
		}(client)
	}
	var success int
	responses := make([]*protocol.BroadcastResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
}

func (sender *sender) answer(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.AnswerResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// answer parallel
	response := make(chan *protocol.AnswerResponse, l)
	for _, client := range clients {
		go func(client *Client) {
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Print(r, "sender.answer")
					sender.log(logger.Fatal, buf)
				}
			}()
			response <- client.Answer(guid, data)
		}(client)
	}
	var success int
	responses := make([]*protocol.AnswerResponse, l)
	for i := 0; i < l; i++ {
		responses[i] = <-response
		if responses[i].Err == nil {
			success++
		}
	}
	close(response)
	return responses, success
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
			sender.cleanNodeAckSlotMap()
			sender.cleanBeaconAckSlotMap()
		case <-sender.context.Done():
			return
		}
	}
}

func (sender *sender) cleanNodeAckSlotMap() {
	sender.nodeAckSlotsRWM.Lock()
	defer sender.nodeAckSlotsRWM.Unlock()
	newMap := make(map[guid.GUID]*roleAckSlot, len(sender.nodeAckSlots))
	for key, nas := range sender.nodeAckSlots {
		if sender.cleanRoleAckSlotMap(nas) {
			newMap[key] = nas
		}
	}
	sender.nodeAckSlots = newMap
}

func (sender *sender) cleanBeaconAckSlotMap() {
	sender.beaconAckSlotsRWM.Lock()
	defer sender.beaconAckSlotsRWM.Unlock()
	newMap := make(map[guid.GUID]*roleAckSlot, len(sender.beaconAckSlots))
	for key, bas := range sender.beaconAckSlots {
		if sender.cleanRoleAckSlotMap(bas) {
			newMap[key] = bas
		}
	}
	sender.beaconAckSlots = newMap
}

// delete zero length map or allocate a new slots map
func (sender *sender) cleanRoleAckSlotMap(ras *roleAckSlot) bool {
	ras.rwm.Lock()
	defer ras.rwm.Unlock()
	l := len(ras.slots)
	if l == 0 {
		return false
	}
	newMap := make(map[guid.GUID]chan struct{}, l)
	for key, ch := range ras.slots {
		newMap[key] = ch
	}
	ras.slots = newMap
	return true
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
	preB protocol.Broadcast
	preR protocol.Answer

	// key
	node   *mNode
	beacon *mBeacon
	aesKey []byte
	aesIV  []byte
	hmac   hash.Hash

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
	// must stop at once, or maybe timeout at the first time.
	sw.timer = time.NewTimer(time.Minute)
	sw.timer.Stop()
	defer sw.timer.Stop()
	var (
		st *sendTask
		at *ackTask
		bt *broadcastTask
		rt *answerTask
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
		case st = <-sw.ctx.sendToNodeTaskQueue:
			sw.handleSendToNodeTask(st)
		case st = <-sw.ctx.sendToBeaconTaskQueue:
			sw.handleSendToBeaconTask(st)
		case at = <-sw.ctx.ackToNodeTaskQueue:
			sw.handleAckToNodeTask(at)
		case at = <-sw.ctx.ackToBeaconTaskQueue:
			sw.handleAckToBeaconTask(at)
		case bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask(bt)
		case rt = <-sw.ctx.answerTaskQueue:
			sw.handleAnswerTask(rt)
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
	sw.msgpack = msgpack.NewEncoder(sw.buffer)
	sw.deflateBuf = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.hash = sha256.New()
	var (
		at *ackTask
		bt *broadcastTask
		rt *answerTask
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
		case at = <-sw.ctx.ackToNodeTaskQueue:
			sw.handleAckToNodeTask(at)
		case at = <-sw.ctx.ackToBeaconTaskQueue:
			sw.handleAckToBeaconTask(at)
		case bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask(bt)
		case rt = <-sw.ctx.answerTaskQueue:
			sw.handleAnswerTask(rt)
		case <-sw.ctx.context.Done():
			return
		}
	}
}

func (sw *senderWorker) handleSendToNodeTask(st *sendTask) {
	result := sw.ctx.sendResultPool.Get().(*protocol.SendResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleSendToNodeTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		st.Result <- result
	}()
	// get Node session key
	sw.node, result.Err = sw.ctx.ctx.database.SelectNode(st.GUID)
	if result.Err != nil {
		return
	}
	sessionKey := sw.node.SessionKey.Get()
	defer sw.node.SessionKey.Put(sessionKey)
	sw.aesKey = sessionKey
	sw.aesIV = sessionKey[:aes.IVSize]
	// set HMAC-SHA256
	hmac := sw.node.HMACPool.Get().(hash.Hash)
	defer sw.node.HMACPool.Put(hmac)
	sw.hmac = hmac
	// pack
	sw.packSendData(st, result)
	if result.Err != nil {
		return
	}
	// send
	wait := sw.ctx.createNodeAckSlot(st.GUID, &sw.preS.GUID)
	defer sw.ctx.destroyNodeAckSlot(st.GUID, &sw.preS.GUID, wait)
	result.Responses, result.Success = sw.ctx.sendToNode(&sw.preS.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToSendToNode
		return
	}
	// wait role acknowledge
	sw.timer.Reset(sw.timeout)
	select {
	case <-wait:
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
	case <-st.ctx.Done():
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
		result.Err = st.ctx.Err()
	case <-sw.timer.C:
		result.Err = ErrSendTimeout
	case <-sw.ctx.context.Done():
		result.Err = ErrSenderClosed
	}
}

func (sw *senderWorker) handleSendToBeaconTask(st *sendTask) {
	result := sw.ctx.sendResultPool.Get().(*protocol.SendResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleSendToBeaconTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		st.Result <- result
	}()
	// get Beacon session key
	sw.beacon, result.Err = sw.ctx.ctx.database.SelectBeacon(st.GUID)
	if result.Err != nil {
		return
	}
	sessionKey := sw.beacon.SessionKey.Get()
	defer sw.beacon.SessionKey.Put(sessionKey)
	sw.aesKey = sessionKey
	sw.aesIV = sessionKey[:aes.IVSize]
	// set HMAC-SHA256
	hmac := sw.beacon.HMACPool.Get().(hash.Hash)
	defer sw.beacon.HMACPool.Put(hmac)
	sw.hmac = hmac
	// check is need to write message to the database
	if !sw.ctx.IsInInteractiveMode(st.GUID) {
		sw.insertBeaconMessage(st, result)
		if result.Err == nil {
			result.Success = 1
		}
		return
	}
	// pack
	sw.packSendData(st, result)
	if result.Err != nil {
		return
	}
	// send
	wait := sw.ctx.createBeaconAckSlot(st.GUID, &sw.preS.GUID)
	defer sw.ctx.destroyBeaconAckSlot(st.GUID, &sw.preS.GUID, wait)
	result.Responses, result.Success = sw.ctx.sendToBeacon(&sw.preS.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToSendToBeacon
		return
	}
	// wait role acknowledge
	sw.timer.Reset(sw.timeout)
	select {
	case <-wait:
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
	case <-st.ctx.Done():
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
		result.Err = st.ctx.Err()
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
			result.Err = ErrTooLargeMessage
			return
		}
		st.Message = sw.deflateBuf.Bytes()
	} else {
		sw.preS.Deflate = 0
	}
	// encrypt message
	sw.preS.Message, result.Err = aes.CBCEncrypt(st.Message, sw.aesKey, sw.aesIV)
	if result.Err != nil {
		return
	}
	// set GUID
	sw.preS.GUID = *sw.ctx.guid.Get()
	sw.preS.RoleGUID = *st.GUID
	// HMAC
	sw.calculateSendHMAC()
	// self validate
	result.Err = sw.preS.Validate()
	if result.Err != nil {
		panic("sender packSendData error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preS.Pack(sw.buffer)
}

func (sw *senderWorker) calculateSendHMAC() {
	sw.hmac.Reset()
	sw.hmac.Write(sw.preS.GUID[:])
	sw.hmac.Write(sw.preS.RoleGUID[:])
	sw.hmac.Write([]byte{sw.preS.Deflate})
	sw.hmac.Write(sw.preS.Message)
	sw.preS.Hash = sw.hmac.Sum(nil)
}

// insertBeaconMessage is used to insert send to Beacon message to
// database, and wait the target Beacon to query it.
func (sw *senderWorker) insertBeaconMessage(st *sendTask, result *protocol.SendResult) {
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
			result.Err = ErrTooLargeMessage
			return
		}
		st.Message = sw.deflateBuf.Bytes()
	} else {
		sw.preS.Deflate = 0
	}
	// encrypt message
	sw.preS.Message, result.Err = aes.CBCEncrypt(st.Message, sw.aesKey, sw.aesIV)
	if result.Err != nil {
		return
	}
	sw.preS.RoleGUID = *st.GUID
	result.Err = sw.ctx.ctx.database.InsertBeaconMessage(&sw.preS)
}

func (sw *senderWorker) handleAckToNodeTask(at *ackTask) {
	result := sw.ctx.ackResultPool.Get().(*protocol.AcknowledgeResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleAckToNodeTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		at.Result <- result
	}()
	// set HMAC-SHA256
	sw.node, result.Err = sw.ctx.ctx.database.SelectNode(at.RoleGUID)
	if result.Err != nil {
		return
	}
	hmac := sw.node.HMACPool.Get().(hash.Hash)
	defer sw.node.HMACPool.Put(hmac)
	sw.hmac = hmac
	// pack
	sw.packAcknowledgeData(at, result)
	if result.Err != nil {
		return
	}
	// acknowledge
	result.Responses, result.Success = sw.ctx.ackToNode(&sw.preA.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToAckToNode
	}
}

func (sw *senderWorker) handleAckToBeaconTask(at *ackTask) {
	result := sw.ctx.ackResultPool.Get().(*protocol.AcknowledgeResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleAckToBeaconTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		at.Result <- result
	}()
	// set HMAC-SHA256
	sw.beacon, result.Err = sw.ctx.ctx.database.SelectBeacon(at.RoleGUID)
	if result.Err != nil {
		return
	}
	hmac := sw.beacon.HMACPool.Get().(hash.Hash)
	defer sw.beacon.HMACPool.Put(hmac)
	sw.hmac = hmac
	// pack
	sw.packAcknowledgeData(at, result)
	if result.Err != nil {
		return
	}
	// acknowledge
	result.Responses, result.Success = sw.ctx.ackToBeacon(&sw.preA.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToAckToBeacon
	}
}

func (sw *senderWorker) packAcknowledgeData(at *ackTask, result *protocol.AcknowledgeResult) {
	sw.preA.GUID = *sw.ctx.guid.Get()
	sw.preA.RoleGUID = *at.RoleGUID
	sw.preA.SendGUID = *at.SendGUID
	// HMAC
	sw.calculateAcknowledgeHMAC()
	// self validate
	result.Err = sw.preA.Validate()
	if result.Err != nil {
		panic("sender packAcknowledgeData error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preA.Pack(sw.buffer)
}

func (sw *senderWorker) calculateAcknowledgeHMAC() {
	sw.hmac.Reset()
	sw.hmac.Write(sw.preA.GUID[:])
	sw.hmac.Write(sw.preA.RoleGUID[:])
	sw.hmac.Write(sw.preA.SendGUID[:])
	sw.preA.Hash = sw.hmac.Sum(nil)
}

func (sw *senderWorker) handleBroadcastTask(bt *broadcastTask) {
	result := sw.ctx.broadcastResultPool.Get().(*protocol.BroadcastResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleBroadcastTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		bt.Result <- result
	}()
	// pack message(interface)
	if bt.MessageI != nil {
		sw.buffer.Reset()
		sw.buffer.Write(sw.rand.Bytes(messages.RandomDataSize))
		sw.buffer.Write(bt.Command)
		if msg, ok := bt.MessageI.([]byte); ok {
			sw.buffer.Write(msg)
		} else {
			result.Err = sw.msgpack.Encode(bt.MessageI)
			if result.Err != nil {
				return
			}
		}
		// don't worry copy, because encrypt
		bt.Message = sw.buffer.Bytes()
	}
	// hash
	sw.hash.Reset()
	sw.hash.Write(bt.Message)
	sw.preB.Hash = sw.hash.Sum(nil)
	// compress message
	if bt.Deflate {
		sw.preB.Deflate = 1
		writer := sw.ctx.deflateWriterPool.Get().(*flate.Writer)
		defer sw.ctx.deflateWriterPool.Put(writer)
		sw.deflateBuf.Reset()
		writer.Reset(sw.deflateBuf)
		_, result.Err = writer.Write(bt.Message)
		if result.Err != nil {
			return
		}
		result.Err = writer.Close()
		if result.Err != nil {
			return
		}
		// check compressed message size
		if sw.deflateBuf.Len() > protocol.MaxFrameSize {
			result.Err = ErrTooLargeMessage
			return
		}
		bt.Message = sw.deflateBuf.Bytes()
	} else {
		sw.preB.Deflate = 0
	}
	// encrypt compressed message
	sw.preB.Message, result.Err = sw.ctx.ctx.global.Encrypt(bt.Message)
	if result.Err != nil {
		return
	}
	// GUID
	sw.preB.GUID = *sw.ctx.guid.Get()
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preB.GUID[:])
	sw.buffer.Write(sw.preB.Hash)
	sw.buffer.WriteByte(sw.preB.Deflate)
	sw.buffer.Write(sw.preB.Message)
	sw.preB.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// self validate
	result.Err = sw.preB.Validate()
	if result.Err != nil {
		panic("sender handleBroadcastTask error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preB.Pack(sw.buffer)
	// broadcast
	result.Responses, result.Success = sw.ctx.broadcast(&sw.preB.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToBroadcast
	}
}

func (sw *senderWorker) handleAnswerTask(rt *answerTask) {
	result := sw.ctx.answerResultPool.Get().(*protocol.AnswerResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleAnswerTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		rt.Result <- result
	}()
	// for set HMAC
	sw.beacon, result.Err = sw.ctx.ctx.database.SelectBeacon(rt.BeaconGUID)
	if result.Err != nil {
		return
	}
	// set answer
	sw.preR.GUID = *sw.ctx.guid.Get()
	sw.preR.BeaconGUID = *rt.BeaconGUID
	sw.preR.Index = rt.Index
	sw.preR.Deflate = rt.Deflate
	sw.preR.Message = rt.Message
	// HMAC
	sw.calculateAnswerHMAC()
	// self validate
	result.Err = sw.preR.Validate()
	if result.Err != nil {
		panic("sender handleAnswerTask error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preR.Pack(sw.buffer)
	// answer
	result.Responses, result.Success = sw.ctx.answer(&sw.preR.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToAnswer
	}
}

func (sw *senderWorker) calculateAnswerHMAC() {
	h := sw.beacon.HMACPool.Get().(hash.Hash)
	defer sw.beacon.HMACPool.Put(h)
	h.Reset()
	h.Write(sw.preR.GUID[:])
	h.Write(sw.preR.BeaconGUID[:])
	h.Write(convert.BEUint64ToBytes(sw.preR.Index))
	h.Write([]byte{sw.preR.Deflate})
	h.Write(sw.preR.Message)
	sw.preR.Hash = h.Sum(nil)
}

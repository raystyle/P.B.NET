package controller

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
	"project/internal/crypto/aes"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

// errors
var (
	ErrNoConnections   = fmt.Errorf("no connections")
	ErrBroadcastFailed = fmt.Errorf("failed to broadcast")
	ErrSendFailed      = fmt.Errorf("failed to send")
	ErrSendTimeout     = fmt.Errorf("send timeout")
	ErrSenderMaxConns  = fmt.Errorf("sender with max connections")
	ErrSenderClosed    = fmt.Errorf("sender closed")
)

// broadcastTask is used to broadcast message to all Nodes
// MessageI will be Encode by msgpack, except MessageI.(type) is []byte
type broadcastTask struct {
	Command  []byte      // for Broadcast
	MessageI interface{} // for Broadcast
	Message  []byte      // for BroadcastFromPlugin
	Result   chan<- *protocol.BroadcastResult
}

// sendTask is used to send message to the target Node or Beacon.
// MessageI will be Encode by msgpack, except MessageI.(type) is []byte.
type sendTask struct {
	Ctx      context.Context
	GUID     *guid.GUID  // receiver role's GUID
	Command  []byte      // for Send
	MessageI interface{} // for Send
	Message  []byte      // for SendFromPlugin
	Result   chan<- *protocol.SendResult
}

// ackTask is used to acknowledge to the Node or Beacon.
// must not use *guid.GUID, because sender.Acknowledge() will not block.
type ackTask struct {
	RoleGUID guid.GUID
	SendGUID guid.GUID
}

// wait role acknowledge
type roleAckSlot struct {
	// key = Send.GUID
	slots map[guid.GUID]chan struct{}
	m     sync.Mutex
}

type sender struct {
	ctx *Ctrl

	maxConns atomic.Value

	sendToNodeTaskQueue   chan *sendTask
	sendToBeaconTaskQueue chan *sendTask
	ackToNodeTaskQueue    chan *ackTask
	ackToBeaconTaskQueue  chan *ackTask
	broadcastTaskQueue    chan *broadcastTask

	sendTaskPool      sync.Pool
	ackTaskPool       sync.Pool
	broadcastTaskPool sync.Pool

	sendDonePool      sync.Pool
	broadcastDonePool sync.Pool

	sendResultPool      sync.Pool
	broadcastResultPool sync.Pool

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

	inClose    int32
	stopSignal chan struct{}
	wg         sync.WaitGroup
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
		clients:               make(map[guid.GUID]*Client, cfg.MaxConns),
		interactive:           make(map[guid.GUID]bool),
		nodeAckSlots:          make(map[guid.GUID]*roleAckSlot),
		beaconAckSlots:        make(map[guid.GUID]*roleAckSlot),
		stopSignal:            make(chan struct{}, 1),
	}

	sender.maxConns.Store(cfg.MaxConns)

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
	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	sender.broadcastDonePool.New = func() interface{} {
		return make(chan *protocol.BroadcastResult, 1)
	}
	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	sender.broadcastResultPool.New = func() interface{} {
		return new(protocol.BroadcastResult)
	}
	sender.ackSlotPool.New = func() interface{} {
		return make(chan struct{}, 1)
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
	sender.wg.Add(1)
	go sender.ackSlotCleaner()
	return sender, nil
}

// GetMaxConns is used to get sender max connection
func (sender *sender) GetMaxConns() int {
	return sender.maxConns.Load().(int)
}

// SetMaxConns is used to set sender max connection
func (sender *sender) SetMaxConns(n int) {
	sender.maxConns.Store(n)
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

func (sender *sender) logf(l logger.Level, format string, log ...interface{}) {
	sender.ctx.logger.Printf(l, "sender", format, log...)
}

func (sender *sender) log(l logger.Level, log ...interface{}) {
	sender.ctx.logger.Println(l, "sender", log...)
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
func (sender *sender) Synchronize(ctx context.Context, guid *guid.GUID, bl *bootstrap.Listener) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	// must simple check firstly
	err := sender.checkNode(guid)
	if err != nil {
		return err
	}
	// create client
	client, err := sender.ctx.NewClient(ctx, bl, guid, func() {
		sender.clientsRWM.Lock()
		defer sender.clientsRWM.Unlock()
		delete(sender.clients, *guid)
	})
	if err != nil {
		return errors.WithMessage(err, "failed to create client")
	}
	// interrupt
	wg := sync.WaitGroup{}
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer func() {
			recover()
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
	err = client.Synchronize()
	if err != nil {
		const format = "failed to start to synchronize\nlistener: %s\n%s\nerror"
		return errors.WithMessagef(err, format, bl, guid.Hex())
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
	success = true
	return nil
}

// Disconnect is used to disconnect Node.
func (sender *sender) Disconnect(guid *guid.GUID) error {
	if client, ok := sender.Clients()[*guid]; ok {
		client.Close()
		return nil
	}
	return errors.Errorf("client doesn't exist\n%s", guid)
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
) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Ctx = ctx
	st.GUID = guid
	st.Command = command
	st.MessageI = message
	st.Result = done
	// send to task queue
	select {
	case sender.sendToNodeTaskQueue <- st:
	case <-sender.stopSignal:
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
) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Ctx = ctx
	st.GUID = guid
	st.Command = command
	st.MessageI = message
	st.Result = done
	// send to task queue
	select {
	case sender.sendToBeaconTaskQueue <- st:
	case <-sender.stopSignal:
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendToNodeFromPlugin is used to send message to the target Node from plugin.
func (sender *sender) SendToNodeFromPlugin(GUID, message []byte) error {
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
	st.Ctx = context.Background()
	st.GUID = g
	st.Message = message
	st.Result = done
	// send to task queue
	select {
	case sender.sendToNodeTaskQueue <- st:
	case <-sender.stopSignal:
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendToBeaconFromPlugin is used to send message to the target Beacon from plugin.
func (sender *sender) SendToBeaconFromPlugin(GUID, message []byte) error {
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
	st.Ctx = context.Background()
	st.GUID = g
	st.Message = message
	st.Result = done
	// send to task queue
	select {
	case sender.sendToBeaconTaskQueue <- st:
	case <-sender.stopSignal:
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// AcknowledgeToNode is used to acknowledge Node that Controller has received this message.
func (sender *sender) AcknowledgeToNode(send *protocol.Send) {
	if sender.isClosed() {
		return
	}
	at := sender.ackTaskPool.Get().(*ackTask)
	at.RoleGUID = send.RoleGUID
	at.SendGUID = send.GUID
	select {
	case sender.ackToNodeTaskQueue <- at:
	case <-sender.stopSignal:
	}
}

// AcknowledgeToBeacon is used to acknowledge Beacon that Controller has received this message.
func (sender *sender) AcknowledgeToBeacon(send *protocol.Send) {
	if sender.isClosed() {
		return
	}
	at := sender.ackTaskPool.Get().(*ackTask)
	at.RoleGUID = send.RoleGUID
	at.SendGUID = send.GUID
	select {
	case sender.ackToBeaconTaskQueue <- at:
	case <-sender.stopSignal:
	}
}

// Broadcast is used to broadcast message to all Nodes.
func (sender *sender) Broadcast(command []byte, message interface{}) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	defer sender.broadcastDonePool.Put(done)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	defer sender.broadcastTaskPool.Put(bt)
	bt.Command = command
	bt.MessageI = message
	bt.Result = done
	// send to task queue
	select {
	case sender.broadcastTaskQueue <- bt:
	case <-sender.stopSignal:
		return ErrSenderClosed
	}
	result := <-done
	defer sender.broadcastResultPool.Put(result)
	return result.Err
}

// BroadcastFromPlugin is used to broadcast message to all Nodes from plugin
func (sender *sender) BroadcastFromPlugin(message []byte) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	defer sender.broadcastDonePool.Put(done)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	defer sender.broadcastTaskPool.Put(bt)
	bt.Message = message
	bt.Result = done
	// send to task queue
	select {
	case sender.broadcastTaskQueue <- bt:
	case <-sender.stopSignal:
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
	nas.m.Lock()
	defer nas.m.Unlock()
	ch := nas.slots[*send]
	if ch != nil {
		select {
		case ch <- struct{}{}:
		case <-sender.stopSignal:
			return
		}
		delete(nas.slots, *send)
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
	bas.m.Lock()
	defer bas.m.Unlock()
	ch := bas.slots[*send]
	if ch != nil {
		select {
		case ch <- struct{}{}:
		case <-sender.stopSignal:
			return
		}
		delete(bas.slots, *send)
	}
}

// Answer is used to
func (sender *sender) Answer() {

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

func (sender *sender) isInInteractiveMode(guid *guid.GUID) bool {
	sender.interactiveRWM.RLock()
	defer sender.interactiveRWM.RUnlock()
	return sender.interactive[*guid]
}

func (sender *sender) DeleteNode(guid *guid.GUID) {
	sender.nodeAckSlotsRWM.Lock()
	defer sender.nodeAckSlotsRWM.Unlock()
	delete(sender.nodeAckSlots, *guid)
}

func (sender *sender) DeleteBeacon(guid *guid.GUID) {
	sender.DisableInteractiveMode(guid)
	sender.beaconAckSlotsRWM.Lock()
	defer sender.beaconAckSlotsRWM.Unlock()
	delete(sender.beaconAckSlots, *guid)
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
	close(sender.stopSignal)
	sender.wg.Wait() // wait all acknowledge task finish
	for {
		// disconnect all sender client
		for _, client := range sender.Clients() {
			client.Close()
		}
		// wait close
		time.Sleep(100 * time.Millisecond)
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

func (sender *sender) createNodeAckSlot(role, send *guid.GUID) (chan struct{}, func()) {
	ch := sender.ackSlotPool.Get().(chan struct{})
	nas := sender.mustGetNodeAckSlot(role)
	nas.m.Lock()
	defer nas.m.Unlock()
	nas.slots[*send] = ch
	return ch, func() {
		nas.m.Lock()
		defer nas.m.Unlock()
		// when read channel timeout, worker call destroy(),
		// the channel maybe has sign, try to clean it.
		select {
		case <-ch:
		default:
		}
		sender.ackSlotPool.Put(ch)
		delete(nas.slots, *send)
	}
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

func (sender *sender) createBeaconAckSlot(role, send *guid.GUID) (chan struct{}, func()) {
	ch := sender.ackSlotPool.Get().(chan struct{})
	bas := sender.mustGetBeaconAckSlot(role)
	bas.m.Lock()
	defer bas.m.Unlock()
	bas.slots[*send] = ch
	return ch, func() {
		bas.m.Lock()
		defer bas.m.Unlock()
		// when read channel timeout, worker call destroy(),
		// the channel maybe has sign, try to clean it.
		select {
		case <-ch:
		default:
		}
		sender.ackSlotPool.Put(ch)
		delete(bas.slots, *send)
	}
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
					b := xpanic.Print(r, "sender.sendToNode")
					sender.log(logger.Fatal, b)
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
					b := xpanic.Print(r, "sender.sendToBeacon")
					sender.log(logger.Fatal, b)
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

func (sender *sender) acknowledgeToNode(
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
					b := xpanic.Print(r, "sender.acknowledgeToNode")
					sender.log(logger.Fatal, b)
				}
			}()
			response <- client.AcknowledgeToNode(guid, data)
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

func (sender *sender) acknowledgeToBeacon(
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
					b := xpanic.Print(r, "sender.acknowledgeToBeacon")
					sender.log(logger.Fatal, b)
				}
			}()
			response <- client.AcknowledgeToBeacon(guid, data)
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
					b := xpanic.Print(r, "sender.broadcast")
					sender.log(logger.Fatal, b)
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
		case <-sender.stopSignal:
			return
		}
	}
}

func (sender *sender) cleanNodeAckSlotMap() {
	sender.nodeAckSlotsRWM.Lock()
	defer sender.nodeAckSlotsRWM.Unlock()
	for key, ras := range sender.nodeAckSlots {
		if sender.cleanRoleAckSlotMap(ras) {
			delete(sender.nodeAckSlots, key)
		}
	}
}

func (sender *sender) cleanBeaconAckSlotMap() {
	sender.beaconAckSlotsRWM.Lock()
	defer sender.beaconAckSlotsRWM.Unlock()
	for key, ras := range sender.beaconAckSlots {
		if sender.cleanRoleAckSlotMap(ras) {
			delete(sender.nodeAckSlots, key)
		}
	}
}

// delete zero length map or allocate a new slots map
func (sender *sender) cleanRoleAckSlotMap(ras *roleAckSlot) bool {
	ras.m.Lock()
	defer ras.m.Unlock()
	if len(ras.slots) == 0 {
		return true
	}
	newMap := make(map[guid.GUID]chan struct{})
	for key, value := range ras.slots {
		newMap[key] = value
	}
	ras.slots = newMap
	return false
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
	preB protocol.Broadcast

	// key
	node   *mNode
	beacon *mBeacon
	aesKey []byte
	aesIV  []byte

	// receive acknowledge timeout
	timer *time.Timer
}

func (sw *senderWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			sw.ctx.log(logger.Fatal, xpanic.Print(r, "senderWorker.Work"))
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
	sw.timer = time.NewTimer(sw.timeout)
	defer sw.timer.Stop()
	var (
		st *sendTask
		at *ackTask
		bt *broadcastTask
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
		case st = <-sw.ctx.sendToNodeTaskQueue:
			sw.handleSendToNodeTask(st)
		case st = <-sw.ctx.sendToBeaconTaskQueue:
			sw.handleSendToBeaconTask(st)
		case at = <-sw.ctx.ackToNodeTaskQueue:
			sw.handleAcknowledgeToNodeTask(at)
		case at = <-sw.ctx.ackToBeaconTaskQueue:
			sw.handleAcknowledgeToBeaconTask(at)
		case bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask(bt)
		case <-sw.ctx.stopSignal:
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
	sw.aesKey = sw.node.SessionKey
	sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	// pack
	if !sw.packSendData(st, result) {
		return
	}
	// send
	wait, destroy := sw.ctx.createNodeAckSlot(st.GUID, &sw.preS.GUID)
	result.Responses, result.Success = sw.ctx.sendToNode(&sw.preS.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrSendFailed
		return
	}
	// wait role acknowledge
	sw.timer.Reset(sw.timeout)
	select {
	case <-wait:
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
		sw.ctx.ackSlotPool.Put(wait)
	case <-sw.timer.C:
		destroy()
		result.Err = ErrSendTimeout
	case <-st.Ctx.Done():
		destroy()
		result.Err = st.Ctx.Err()
	case <-sw.ctx.stopSignal:
		result.Err = ErrSenderClosed
	}
}

// TODO finish query mode
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
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	// pack
	if !sw.packSendData(st, result) {
		return
	}
	// check is need to write message to the database
	if !sw.ctx.isInInteractiveMode(st.GUID) {
		result.Err = sw.ctx.ctx.database.InsertBeaconMessage(st.GUID, sw.buffer)
		if result.Err == nil {
			result.Success = 1
		}
		return
	}
	// send
	wait, destroy := sw.ctx.createBeaconAckSlot(st.GUID, &sw.preS.GUID)
	result.Responses, result.Success = sw.ctx.sendToBeacon(&sw.preS.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrSendFailed
		return
	}
	// wait role acknowledge
	sw.timer.Reset(sw.timeout)
	select {
	case <-wait:
		if !sw.timer.Stop() {
			<-sw.timer.C
		}
		sw.ctx.ackSlotPool.Put(wait)
	case <-sw.timer.C:
		destroy()
		result.Err = ErrSendTimeout
	case <-st.Ctx.Done():
		destroy()
		result.Err = st.Ctx.Err()
	case <-sw.ctx.stopSignal:
		result.Err = ErrSenderClosed
	}
}

func (sw *senderWorker) packSendData(st *sendTask, result *protocol.SendResult) bool {
	// pack message(interface)
	if st.MessageI != nil {
		sw.buffer.Reset()
		sw.buffer.Write(st.Command)
		if msg, ok := st.MessageI.([]byte); ok {
			sw.buffer.Write(msg)
		} else {
			result.Err = sw.msgpack.Encode(st.MessageI)
			if result.Err != nil {
				return false
			}
		}
		// don't worry copy, because encrypt
		st.Message = sw.buffer.Bytes()
	}
	// check message size
	if len(st.Message) > protocol.MaxFrameSize {
		result.Err = protocol.ErrTooBigFrame
		return false
	}
	// encrypt
	sw.preS.Message, result.Err = aes.CBCEncrypt(st.Message, sw.aesKey, sw.aesIV)
	if result.Err != nil {
		return false
	}
	// set GUID
	sw.preS.GUID = *sw.ctx.guid.Get()
	sw.preS.RoleGUID = *st.GUID
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
	return true
}

func (sw *senderWorker) handleAcknowledgeToNodeTask(at *ackTask) {
	defer func() {
		if r := recover(); r != nil {
			b := xpanic.Print(r, "senderWorker.handleAcknowledgeToNodeTask")
			sw.ctx.log(logger.Fatal, b)
		}
		sw.ctx.ackTaskPool.Put(at)
	}()
	sw.packAcknowledgeData(at)
	sw.ctx.acknowledgeToNode(&sw.preA.GUID, sw.buffer)
}

func (sw *senderWorker) handleAcknowledgeToBeaconTask(at *ackTask) {
	defer func() {
		if r := recover(); r != nil {
			b := xpanic.Print(r, "senderWorker.handleAcknowledgeToBeaconTask")
			sw.ctx.log(logger.Fatal, b)
		}
		sw.ctx.ackTaskPool.Put(at)
	}()
	sw.packAcknowledgeData(at)
	sw.ctx.acknowledgeToBeacon(&sw.preA.GUID, sw.buffer)
}

func (sw *senderWorker) packAcknowledgeData(at *ackTask) {
	sw.preA.GUID = *sw.ctx.guid.Get()
	sw.preA.RoleGUID = at.RoleGUID
	sw.preA.SendGUID = at.SendGUID
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
	// check message size
	if len(bt.Message) > protocol.MaxFrameSize {
		result.Err = protocol.ErrTooBigFrame
		return
	}
	// encrypt
	sw.preB.Message, result.Err = sw.ctx.ctx.global.Encrypt(bt.Message)
	if result.Err != nil {
		return
	}
	// GUID
	sw.preB.GUID = *sw.ctx.guid.Get()
	// hash
	sw.hash.Reset()
	sw.hash.Write(bt.Message)
	sw.preB.Hash = sw.hash.Sum(nil)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preB.GUID[:])
	sw.buffer.Write(sw.preB.Hash)
	sw.buffer.Write(sw.preB.Message)
	sw.preB.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// self validate
	sw.err = sw.preB.Validate()
	if sw.err != nil {
		panic("sender internal error: " + sw.err.Error())
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
		result.Err = ErrBroadcastFailed
	}
}

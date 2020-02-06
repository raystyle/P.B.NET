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

// MessageI will be Encode by msgpack, except MessageI.(type) is []byte
type broadcastTask struct {
	Command  []byte      // for Broadcast
	MessageI interface{} // for Broadcast
	Message  []byte      // for BroadcastFromPlugin
	Result   chan<- *protocol.BroadcastResult
}

// MessageI will be Encode by msgpack, except MessageI.(type) is []byte
type sendTask struct {
	Role     protocol.Role // receiver role
	GUID     *guid.GUID    // receiver role's GUID
	Command  []byte        // for Send
	MessageI interface{}   // for Send
	Message  []byte        // for SendFromPlugin
	Result   chan<- *protocol.SendResult
}

// must not use *guid.GUID, sender.Acknowledge() will not block
type ackTask struct {
	Role     protocol.Role
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
	ctx *CTRL

	maxConns atomic.Value

	broadcastTaskQueue chan *broadcastTask
	sendTaskQueue      chan *sendTask
	ackTaskQueue       chan *ackTask

	broadcastTaskPool sync.Pool
	sendTaskPool      sync.Pool
	ackTaskPool       sync.Pool

	broadcastDonePool sync.Pool
	sendDonePool      sync.Pool

	broadcastResultPool sync.Pool
	sendResultPool      sync.Pool

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

func newSender(ctx *CTRL, config *Config) (*sender, error) {
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
		ctx:                ctx,
		broadcastTaskQueue: make(chan *broadcastTask, cfg.QueueSize),
		sendTaskQueue:      make(chan *sendTask, cfg.QueueSize),
		ackTaskQueue:       make(chan *ackTask, cfg.QueueSize),
		clients:            make(map[guid.GUID]*Client, cfg.MaxConns),
		interactive:        make(map[guid.GUID]bool),
		nodeAckSlots:       make(map[guid.GUID]*roleAckSlot),
		beaconAckSlots:     make(map[guid.GUID]*roleAckSlot),
		stopSignal:         make(chan struct{}, 1),
	}

	sender.maxConns.Store(cfg.MaxConns)

	// initialize sync pool
	sender.broadcastTaskPool.New = func() interface{} {
		return new(broadcastTask)
	}
	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.ackTaskPool.New = func() interface{} {
		return new(ackTask)
	}
	sender.broadcastDonePool.New = func() interface{} {
		return make(chan *protocol.BroadcastResult, 1)
	}
	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	sender.broadcastResultPool.New = func() interface{} {
		return new(protocol.BroadcastResult)
	}
	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
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

// Synchronize is used to connect a node listener and start synchronize.
func (sender *sender) Synchronize(ctx context.Context, guid *guid.GUID, bl *bootstrap.Listener) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	sender.clientsRWM.Lock()
	defer sender.clientsRWM.Unlock()
	if len(sender.clients) >= sender.GetMaxConns() {
		return ErrSenderMaxConns
	}
	if _, ok := sender.clients[*guid]; ok {
		const format = "already connected the target node\n%s"
		return errors.Errorf(format, guid.Hex())
	}
	// connect
	client, err := sender.ctx.NewClient(ctx, bl, guid, func() {
		sender.clientsRWM.Lock()
		defer sender.clientsRWM.Unlock()
		delete(sender.clients, *guid)
	})
	if err != nil {
		const format = "failed to connect node\nlistener: %s\n%s"
		return errors.WithMessagef(err, format, bl, guid.Hex())
	}
	err = client.Synchronize()
	if err != nil {
		const format = "failed to start synchronize\nlistener: %s\n%s"
		return errors.WithMessagef(err, format, bl, guid.Hex())
	}
	sender.clients[*guid] = client
	return nil
}

// Disconnect is used to disconnect node, guid is hex, upper
func (sender *sender) Disconnect(guid *guid.GUID) error {
	if client, ok := sender.Clients()[*guid]; ok {
		client.Close()
		return nil
	}
	return errors.Errorf("client doesn't exist\n%s", guid)
}

// Broadcast is used to broadcast message to all Nodes
// message will not be saved
func (sender *sender) Broadcast(cmd []byte, msg interface{}) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	defer sender.broadcastDonePool.Put(done)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	defer sender.broadcastTaskPool.Put(bt)
	bt.Command = cmd
	bt.MessageI = msg
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
func (sender *sender) BroadcastFromPlugin(msg []byte) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.broadcastDonePool.Get().(chan *protocol.BroadcastResult)
	defer sender.broadcastDonePool.Put(done)
	bt := sender.broadcastTaskPool.Get().(*broadcastTask)
	defer sender.broadcastTaskPool.Put(bt)
	bt.Message = msg
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

// Send is used to send message to Node or Beacon. if Beacon is not in interactive mode,
// message will saved to database, and wait Beacon to query it, controller will answer
func (sender *sender) Send(role protocol.Role, guid *guid.GUID, cmd []byte, msg interface{}) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	// check role
	switch role {
	case protocol.Node, protocol.Beacon:
	default:
		panic("invalid role")
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Role = role
	st.GUID = guid
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

// SendFromPlugin is used to send message to Node or Beacon from plugin
func (sender *sender) SendFromPlugin(role protocol.Role, guid *guid.GUID, msg []byte) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	// check role
	switch role {
	case protocol.Node, protocol.Beacon:
	default:
		return role
	}
	done := sender.sendDonePool.Get().(chan *protocol.SendResult)
	defer sender.sendDonePool.Put(done)
	st := sender.sendTaskPool.Get().(*sendTask)
	defer sender.sendTaskPool.Put(st)
	st.Role = role
	st.GUID = guid
	st.Message = msg
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

// Acknowledge is used to acknowledge Role that controller has received this message
func (sender *sender) Acknowledge(role protocol.Role, send *protocol.Send) {
	if sender.isClosed() {
		return
	}
	// check role
	switch role {
	case protocol.Node, protocol.Beacon:
	default:
		panic("invalid role")
	}
	at := sender.ackTaskPool.Get().(*ackTask)
	at.Role = role
	at.RoleGUID = send.RoleGUID
	at.SendGUID = send.GUID
	select {
	case sender.ackTaskQueue <- at:
	case <-sender.stopSignal:
	}
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

func (sender *sender) Answer() {

}

func (sender *sender) EnableInteractiveMode(guid *guid.GUID) {
	sender.interactiveRWM.Lock()
	defer sender.interactiveRWM.Unlock()
	sender.interactive[*guid] = true
}

func (sender *sender) DisableInteractiveStatus(guid *guid.GUID) {
	sender.interactiveRWM.Lock()
	defer sender.interactiveRWM.Unlock()
	delete(sender.interactive, *guid)
}

func (sender *sender) isInInteractiveMode(guid *guid.GUID) bool {
	sender.interactiveRWM.RLock()
	defer sender.interactiveRWM.RUnlock()
	return sender.interactive[*guid]
}

func (sender *sender) Close() {
	atomic.StoreInt32(&sender.inClose, 1)
	// TODO wait acknowledge handle
	for {
		if len(sender.ackTaskQueue) == 0 {
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
	resp := make(chan *protocol.BroadcastResponse)
	for _, client := range clients {
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.broadcast")
					sender.log(logger.Fatal, err)
				}
			}()
			resp <- c.Broadcast(guid, data)
		}(client)
	}
	var success int
	response := make([]*protocol.BroadcastResponse, l)
	for i := 0; i < l; i++ {
		response[i] = <-resp
		if response[i].Err == nil {
			success++
		}
	}
	close(resp)
	return response, success
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
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.sendToNode")
					sender.log(logger.Fatal, err)
				}
			}()
			response <- c.SendToNode(guid, data)
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
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.sendToBeacon")
					sender.log(logger.Fatal, err)
				}
			}()
			response <- c.SendToBeacon(guid, data)
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
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.acknowledgeToNode")
					sender.log(logger.Fatal, err)
				}
			}()
			response <- c.AcknowledgeToNode(guid, data)
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
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.acknowledgeToBeacon")
					sender.log(logger.Fatal, err)
				}
			}()
			response <- c.AcknowledgeToBeacon(guid, data)
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
			err := xpanic.Error(r, "senderWorker.Work")
			sw.ctx.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			go sw.Work()
		} else {
			sw.timer.Stop()
			sw.ctx.wg.Done()
		}
	}()
	sw.buffer = bytes.NewBuffer(make([]byte, protocol.SendMinBufferSize))
	sw.msgpack = msgpack.NewEncoder(sw.buffer)
	sw.hash = sha256.New()
	sw.timer = time.NewTimer(sw.timeout)
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
		case st = <-sw.ctx.sendTaskQueue:
			sw.handleSendTask(st)
		case at = <-sw.ctx.ackTaskQueue:
			sw.handleAcknowledgeTask(at)
		case bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask(bt)
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
	switch st.Role {
	case protocol.Node:
		sw.node, result.Err = sw.ctx.ctx.database.SelectNode(st.GUID)
		if result.Err != nil {
			return
		}
		sw.aesKey = sw.node.SessionKey
		sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	case protocol.Beacon:
		sw.beacon, result.Err = sw.ctx.ctx.database.SelectBeacon(st.GUID)
		if result.Err != nil {
			return
		}
		sw.aesKey = sw.beacon.SessionKey
		sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	default:
		panic("invalid st.Role")
	}
	// encrypt
	sw.preS.Message, result.Err = aes.CBCEncrypt(st.Message, sw.aesKey, sw.aesIV)
	if result.Err != nil {
		return
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

	// TODO query
	// check is need to write message to the database
	if st.Role == protocol.Beacon && !sw.ctx.isInInteractiveMode(st.GUID) {
		result.Err = sw.ctx.ctx.database.InsertBeaconMessage(st.GUID, sw.buffer)
		if result.Err == nil {
			result.Success = 1
		}
		return
	}
	// start send
	var (
		// only read, but in sync.Pool, not use <- chan struct{}
		wait chan struct{}
		// if send time out, need call it
		destroy func()
	)
	switch st.Role {
	case protocol.Node:
		wait, destroy = sw.ctx.createNodeAckSlot(st.GUID, &sw.preS.GUID)
		result.Responses, result.Success = sw.ctx.sendToNode(&sw.preS.GUID, sw.buffer)
	case protocol.Beacon:
		wait, destroy = sw.ctx.createBeaconAckSlot(st.GUID, &sw.preS.GUID)
		result.Responses, result.Success = sw.ctx.sendToBeacon(&sw.preS.GUID, sw.buffer)
	default:
		panic("invalid st.Role")
	}
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
	case <-sw.ctx.stopSignal:
		result.Err = ErrSenderClosed
	}
}

func (sw *senderWorker) handleAcknowledgeTask(at *ackTask) {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleAcknowledgeTask")
			sw.ctx.log(logger.Fatal, err)
		}
		sw.ctx.ackTaskPool.Put(at)
	}()
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
	// TODO try, if failed
	switch at.Role {
	case protocol.Node:
		sw.ctx.acknowledgeToNode(&sw.preA.GUID, sw.buffer)
	case protocol.Beacon:
		sw.ctx.acknowledgeToBeacon(&sw.preA.GUID, sw.buffer)
	default:
		panic("invalid at.Role")
	}
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

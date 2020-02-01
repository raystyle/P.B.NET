package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"
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
	GUID     []byte        // receiver role's GUID
	Command  []byte        // for Send
	MessageI interface{}   // for Send
	Message  []byte        // for SendFromPlugin
	Result   chan<- *protocol.SendResult
}

type ackTask struct {
	Role     protocol.Role
	RoleGUID []byte
	SendGUID []byte
}

// wait role acknowledge
type roleAckSlot struct {
	// key = hex(send GUID) lower
	slots map[string]chan struct{}
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

	// key = hex(GUID) upper
	clients    map[string]*Client
	clientsRWM sync.RWMutex

	// check beacon is in interactive mode
	// key = hex(GUID) lower
	interactive    map[string]bool
	interactiveRWM sync.RWMutex

	// receive acknowledge
	// key = hex(role GUID) lower
	nodeAckSlots      map[string]*roleAckSlot
	nodeAckSlotsRWM   sync.RWMutex
	beaconAckSlots    map[string]*roleAckSlot
	beaconAckSlotsRWM sync.RWMutex

	guid *guid.Generator

	inClose int32
	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
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
		clients:            make(map[string]*Client, cfg.MaxConns),
		interactive:        make(map[string]bool),
		nodeAckSlots:       make(map[string]*roleAckSlot),
		beaconAckSlots:     make(map[string]*roleAckSlot),
	}

	sender.maxConns.Store(cfg.MaxConns)

	sender.broadcastTaskPool.New = func() interface{} {
		return new(broadcastTask)
	}
	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.ackTaskPool.New = func() interface{} {
		return &ackTask{
			RoleGUID: make([]byte, guid.Size),
			SendGUID: make([]byte, guid.Size),
		}
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
	sender.guid = guid.New(cfg.QueueSize, ctx.global.Now)
	sender.context, sender.cancel = context.WithCancel(context.Background())

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

func (sender *sender) isClosed() bool {
	return atomic.LoadInt32(&sender.inClose) != 0
}

// Connect is used to connect node for sync message
func (sender *sender) Connect(listener *bootstrap.Listener, guid []byte) error {
	return sender.ConnectWithContext(sender.context, listener, guid)
}

// ConnectWithContext is used to connect node listener with context
func (sender *sender) ConnectWithContext(
	ctx context.Context,
	listener *bootstrap.Listener,
	guid []byte,
) error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	sender.clientsRWM.Lock()
	defer sender.clientsRWM.Unlock()
	if len(sender.clients) >= sender.GetMaxConns() {
		return ErrSenderMaxConns
	}
	key := strings.ToUpper(hex.EncodeToString(guid))
	if _, ok := sender.clients[key]; ok {
		return errors.Errorf("connect the same node listener %s", listener)
	}
	client, err := sender.ctx.NewClient(ctx, listener, guid, func() {
		sender.clientsRWM.Lock()
		defer sender.clientsRWM.Unlock()
		delete(sender.clients, key)
	})
	if err != nil {
		return errors.WithMessage(err, "failed to connect node listener")
	}
	err = client.Synchronize()
	if err != nil {
		return err
	}
	sender.clients[key] = client
	sender.logf(logger.Info, "connect node listener: %s", listener)
	return nil
}

// Disconnect is used to disconnect node, guid is hex, upper
func (sender *sender) Disconnect(guid string) error {
	if client, ok := sender.Clients()[guid]; ok {
		client.Close()
		return nil
	}
	return errors.Errorf("client %s doesn't exist", guid)
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
	case <-sender.context.Done():
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
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.broadcastResultPool.Put(result)
	return result.Err
}

// Send is used to send message to Node or Beacon.
// if Beacon is not in interactive mode, message
// will saved to database, and wait Beacon to query.
func (sender *sender) Send(role protocol.Role, guid, cmd []byte, msg interface{}) error {
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
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// SendFromPlugin is used to send message to Node or Beacon from plugin
func (sender *sender) SendFromPlugin(role protocol.Role, guid []byte, msg []byte) error {
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
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.sendResultPool.Put(result)
	return result.Err
}

// Acknowledge is used to acknowledge Role that
// controller has received this message
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
	// must copy
	at.Role = role
	copy(at.RoleGUID, send.RoleGUID)
	copy(at.SendGUID, send.GUID)
	select {
	case sender.ackTaskQueue <- at:
	case <-sender.context.Done():
	}
}

func (sender *sender) HandleNodeAcknowledge(role, send string) {
	sender.nodeAckSlotsRWM.RLock()
	defer sender.nodeAckSlotsRWM.RUnlock()
	as, ok := sender.nodeAckSlots[role]
	if !ok {
		return
	}
	as.m.Lock()
	defer as.m.Unlock()
	c := as.slots[send]
	if c != nil {
		close(c)
		delete(as.slots, send)
	}
}

func (sender *sender) HandleBeaconAcknowledge(role, send string) {
	sender.beaconAckSlotsRWM.RLock()
	defer sender.beaconAckSlotsRWM.RUnlock()
	as, ok := sender.beaconAckSlots[role]
	if !ok {
		return
	}
	as.m.Lock()
	defer as.m.Unlock()
	c := as.slots[send]
	if c != nil {
		close(c)
		delete(as.slots, send)
	}
}

// send guid hex
func (sender *sender) createNodeAckSlot(role, send string) (<-chan struct{}, func()) {
	sender.nodeAckSlotsRWM.Lock()
	defer sender.nodeAckSlotsRWM.Unlock()
	as, ok := sender.nodeAckSlots[role]
	if !ok {
		sender.nodeAckSlots[role] = &roleAckSlot{
			slots: make(map[string]chan struct{}),
		}
		as = sender.nodeAckSlots[role]
	}
	as.slots[send] = make(chan struct{})
	return as.slots[send], func() {
		as.m.Lock()
		defer as.m.Unlock()
		delete(as.slots, send)
	}
}

// send guid hex
func (sender *sender) createBeaconAckSlot(role, send string) (<-chan struct{}, func()) {
	sender.beaconAckSlotsRWM.Lock()
	defer sender.beaconAckSlotsRWM.Unlock()
	as, ok := sender.beaconAckSlots[role]
	if !ok {
		sender.beaconAckSlots[role] = &roleAckSlot{
			slots: make(map[string]chan struct{}),
		}
		as = sender.beaconAckSlots[role]
	}
	as.slots[send] = make(chan struct{})
	return as.slots[send], func() {
		as.m.Lock()
		defer as.m.Unlock()
		delete(as.slots, send)
	}
}

func (sender *sender) Answer() {

}

func (sender *sender) SetInteractiveMode(guid string) {
	sender.interactiveRWM.Lock()
	defer sender.interactiveRWM.Unlock()
	sender.interactive[strings.ToLower(guid)] = true
}

func (sender *sender) DeleteInteractiveStatus(guid string) {
	sender.interactiveRWM.Lock()
	defer sender.interactiveRWM.Unlock()
	delete(sender.interactive, strings.ToLower(guid))
}

func (sender *sender) isInInteractiveMode(guid string) bool {
	sender.interactiveRWM.RLock()
	defer sender.interactiveRWM.RUnlock()
	return sender.interactive[guid]
}

func (sender *sender) Clients() map[string]*Client {
	sender.clientsRWM.RLock()
	defer sender.clientsRWM.RUnlock()
	clients := make(map[string]*Client, len(sender.clients))
	for key, client := range sender.clients {
		clients[key] = client
	}
	return clients
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
	sender.cancel()
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

func (sender *sender) logf(l logger.Level, format string, log ...interface{}) {
	sender.ctx.logger.Printf(l, "sender", format, log...)
}

func (sender *sender) log(l logger.Level, log ...interface{}) {
	sender.ctx.logger.Println(l, "sender", log...)
}

func (sender *sender) broadcast(guid, message []byte) ([]*protocol.BroadcastResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// broadcast parallel
	resp := make(chan *protocol.BroadcastResponse)
	for _, c := range clients {
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.broadcast")
					sender.log(logger.Fatal, err)
				}
			}()
			resp <- c.Broadcast(guid, message)
		}(c)
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

func (sender *sender) sendToNode(guid, message []byte) ([]*protocol.SendResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// send parallel
	resp := make(chan *protocol.SendResponse)
	for _, c := range clients {
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.sendToNode")
					sender.log(logger.Fatal, err)
				}
			}()
			resp <- c.SendToNode(guid, message)
		}(c)
	}
	var success int
	response := make([]*protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		response[i] = <-resp
		if response[i].Err == nil {
			success++
		}
	}
	close(resp)
	return response, success
}

func (sender *sender) sendToBeacon(guid, message []byte) ([]*protocol.SendResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// send parallel
	resp := make(chan *protocol.SendResponse)
	for _, c := range clients {
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.sendToBeacon")
					sender.log(logger.Fatal, err)
				}
			}()
			resp <- c.SendToBeacon(guid, message)
		}(c)
	}
	var success int
	response := make([]*protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		response[i] = <-resp
		if response[i].Err == nil {
			success++
		}
	}
	close(resp)
	return response, success
}

func (sender *sender) acknowledgeToNode(guid, data []byte) ([]*protocol.AcknowledgeResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// acknowledge parallel
	resp := make(chan *protocol.AcknowledgeResponse, l)
	for _, c := range clients {
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.acknowledgeToNode")
					sender.log(logger.Fatal, err)
				}
			}()
			resp <- c.AcknowledgeToNode(guid, data)
		}(c)
	}
	var success int
	response := make([]*protocol.AcknowledgeResponse, l)
	for i := 0; i < l; i++ {
		response[i] = <-resp
		if response[i].Err == nil {
			success++
		}
	}
	close(resp)
	return response, success
}

func (sender *sender) acknowledgeToBeacon(guid, data []byte) ([]*protocol.AcknowledgeResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// acknowledge parallel
	resp := make(chan *protocol.AcknowledgeResponse, l)
	for _, c := range clients {
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					err := xpanic.Error(r, "sender.acknowledgeToBeacon")
					sender.log(logger.Fatal, err)
				}
			}()
			resp <- c.AcknowledgeToBeacon(guid, data)
		}(c)
	}
	var success int
	response := make([]*protocol.AcknowledgeResponse, l)
	for i := 0; i < l; i++ {
		response[i] = <-resp
		if response[i].Err == nil {
			success++
		}
	}
	close(resp)
	return response, success
}

type senderWorker struct {
	ctx *sender

	timeout       time.Duration
	maxBufferSize int

	// runtime
	buffer   *bytes.Buffer
	msgpack  *msgpack.Encoder
	hex      io.Writer
	hash     hash.Hash
	roleGUID string
	err      error

	// prepare task objects
	preB protocol.Broadcast
	preS protocol.Send
	preA protocol.Acknowledge

	// key
	node   *mNode
	beacon *mBeacon
	aesKey []byte
	aesIV  []byte

	// receive acknowledge timeout
	tHex  []byte
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
	sw.hex = hex.NewEncoder(sw.buffer)
	sw.hash = sha256.New()
	sw.tHex = make([]byte, 2*guid.Size)
	sw.timer = time.NewTimer(sw.timeout)
	var (
		bt *broadcastTask
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
		case at = <-sw.ctx.ackTaskQueue:
			sw.handleAcknowledgeTask(at)
		case st = <-sw.ctx.sendTaskQueue:
			sw.handleSendTask(st)
		case bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask(bt)
		case <-sw.ctx.context.Done():
			return
		}
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
	sw.preA.GUID = sw.ctx.guid.Get()
	sw.preA.RoleGUID = at.RoleGUID
	sw.preA.SendGUID = at.SendGUID
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preA.GUID)
	sw.buffer.Write(sw.preA.RoleGUID)
	sw.buffer.Write(sw.preA.SendGUID)
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
		sw.ctx.acknowledgeToNode(sw.preA.GUID, sw.buffer.Bytes())
	case protocol.Beacon:
		sw.ctx.acknowledgeToBeacon(sw.preA.GUID, sw.buffer.Bytes())
	default:
		panic("invalid at.Role")
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
	// role GUID string
	sw.buffer.Reset()
	_, _ = sw.hex.Write(st.GUID)
	sw.roleGUID = sw.buffer.String()
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
	sw.preS.GUID = sw.ctx.guid.Get()
	sw.preS.RoleGUID = st.GUID
	// hash
	sw.hash.Reset()
	sw.hash.Write(st.Message)
	sw.preS.Hash = sw.hash.Sum(nil)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preS.GUID)
	sw.buffer.Write(sw.preS.RoleGUID)
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
	if st.Role == protocol.Beacon && !sw.ctx.isInInteractiveMode(sw.roleGUID) {
		result.Err = sw.ctx.ctx.database.InsertBeaconMessage(st.GUID, sw.buffer.Bytes())
		if result.Err == nil {
			result.Success = 1
		}
		return
	}
	// send
	hex.Encode(sw.tHex, sw.preS.GUID) // calculate send guid
	var (
		wait    <-chan struct{}
		destroy func()
	)
	switch st.Role {
	case protocol.Node:
		wait, destroy = sw.ctx.createNodeAckSlot(sw.roleGUID, string(sw.tHex))
		result.Responses, result.Success = sw.ctx.sendToNode(sw.preS.GUID, sw.buffer.Bytes())
	case protocol.Beacon:
		wait, destroy = sw.ctx.createBeaconAckSlot(sw.roleGUID, string(sw.tHex))
		result.Responses, result.Success = sw.ctx.sendToBeacon(sw.preS.GUID, sw.buffer.Bytes())
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
	if !sw.timer.Stop() {
		<-sw.timer.C
	}
	sw.timer.Reset(sw.timeout)
	select {
	case <-wait:
	case <-sw.timer.C:
		destroy()
		result.Err = ErrSendTimeout
	case <-sw.ctx.context.Done():
		result.Err = ErrSenderClosed
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
	sw.preB.GUID = sw.ctx.guid.Get()
	// hash
	sw.hash.Reset()
	sw.hash.Write(bt.Message)
	sw.preB.Hash = sw.hash.Sum(nil)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preB.GUID)
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
	result.Responses, result.Success = sw.ctx.broadcast(sw.preB.GUID, sw.buffer.Bytes())
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrBroadcastFailed
	}
}

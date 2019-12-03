package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
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

var (
	ErrBroadcastFailed = fmt.Errorf("failed to broadcast")
	ErrSendFailed      = fmt.Errorf("failed to send")
	ErrSenderMaxConns  = fmt.Errorf("sender with max connections")
	ErrSenderClosed    = fmt.Errorf("sender closed")
)

type broadcastTask struct {
	Command  []byte      // for Broadcast
	MessageI interface{} // for Broadcast
	Message  []byte      // for BroadcastFromPlugin
	Result   chan<- *protocol.BroadcastResult
}

type sendTask struct {
	Role     protocol.Role // receiver role
	GUID     []byte        // receiver role's GUID
	Command  []byte        // for Send
	MessageI interface{}   // for Send
	Message  []byte        // for SendFromPlugin
	Result   chan<- *protocol.SendResult
}

type acknowledgeTask struct {
	Role     protocol.Role
	RoleGUID []byte
	SendGUID []byte
}

type sender struct {
	ctx *CTRL

	maxConns atomic.Value

	broadcastTaskQueue   chan *broadcastTask
	sendTaskQueue        chan *sendTask
	acknowledgeTaskQueue chan *acknowledgeTask

	broadcastTaskPool   sync.Pool
	sendTaskPool        sync.Pool
	acknowledgeTaskPool sync.Pool

	broadcastDonePool sync.Pool
	sendDonePool      sync.Pool

	broadcastResultPool sync.Pool
	sendResultPool      sync.Pool

	guid *guid.GUID

	// key = hex(guid)
	clients    map[string]*client
	clientsRWM sync.RWMutex

	// check beacon is in interactive mode
	interactive    map[string]bool // key = base64(Beacon GUID)
	interactiveRWM sync.RWMutex

	closing int32
	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newSender(ctx *CTRL, config *Config) (*sender, error) {
	cfg := config.Sender

	// check config
	if cfg.Worker < 4 {
		return nil, errors.New("sender worker number must >= 4")
	}
	if cfg.QueueSize < 512 {
		return nil, errors.New("sender queue size >= 512")
	}
	if cfg.MaxConns < 1 {
		return nil, errors.New("sender max conns >= 1")
	}

	sender := &sender{
		ctx:                  ctx,
		broadcastTaskQueue:   make(chan *broadcastTask, cfg.QueueSize),
		sendTaskQueue:        make(chan *sendTask, cfg.QueueSize),
		acknowledgeTaskQueue: make(chan *acknowledgeTask, cfg.QueueSize),
	}

	sender.maxConns.Store(cfg.MaxConns)

	// init task sync pool
	sender.broadcastTaskPool.New = func() interface{} {
		return new(broadcastTask)
	}
	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.acknowledgeTaskPool.New = func() interface{} {
		return &acknowledgeTask{
			RoleGUID: make([]byte, guid.Size),
			SendGUID: make([]byte, guid.Size),
		}
	}
	// init done sync pool
	sender.broadcastDonePool.New = func() interface{} {
		return make(chan *protocol.BroadcastResult, 1)
	}
	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	// init result sync pool
	sender.broadcastResultPool.New = func() interface{} {
		return new(protocol.BroadcastResult)
	}
	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	sender.guid = guid.New(16*cfg.QueueSize, ctx.global.Now)
	sender.context, sender.cancel = context.WithCancel(context.Background())
	// start sender workers
	for i := 0; i < cfg.Worker; i++ {
		worker := senderWorker{
			ctx:           sender,
			maxBufferSize: cfg.MaxBufferSize,
		}
		sender.wg.Add(1)
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

func (sender *sender) isClosing() bool {
	return atomic.LoadInt32(&sender.closing) != 0
}

// Connect is used to connect node for sync message
func (sender *sender) Connect(node *bootstrap.Node, guid []byte) error {
	if sender.isClosing() {
		return ErrSenderClosed
	}
	sender.clientsRWM.Lock()
	defer sender.clientsRWM.Unlock()
	if len(sender.clients) >= sender.GetMaxConns() {
		return ErrSenderMaxConns
	}
	key := strings.ToUpper(hex.EncodeToString(guid))
	if _, ok := sender.clients[key]; ok {
		return errors.Errorf("connect the same node %s %s", node.Mode, node.Address)
	}
	client, err := newClient(sender.ctx, sender.context, node, guid, func() {
		sender.clientsRWM.Lock()
		defer sender.clientsRWM.Unlock()
		delete(sender.clients, key)
	})
	if err != nil {
		return errors.WithMessage(err, "failed to connect node")
	}
	err = client.Sync()
	if err != nil {
		return err
	}
	sender.clients[key] = client
	sender.logf(logger.Info, "connect node %s %s", node.Mode, node.Address)
	return nil
}

// Disconnect is used to disconnect node, guid is hex, upper
func (sender *sender) Disconnect(guid string) error {
	if client, ok := sender.GetClients()[guid]; ok {
		client.Close()
		return nil
	}
	return errors.Errorf("client %s doesn't exist", guid)
}

// Broadcast is used to broadcast message to all Nodes
// message will not be saved
func (sender *sender) Broadcast(cmd []byte, msg interface{}) error {
	if sender.isClosing() {
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
	if sender.isClosing() {
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
	if sender.isClosing() {
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
	if sender.isClosing() {
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
	if sender.isClosing() {
		return
	}
	// check role
	switch role {
	case protocol.Node, protocol.Beacon:
	default:
		panic("invalid role")
	}
	at := sender.acknowledgeTaskPool.Get().(*acknowledgeTask)
	// must copy
	at.Role = role
	copy(at.RoleGUID, send.RoleGUID)
	copy(at.SendGUID, send.GUID)
	select {
	case sender.acknowledgeTaskQueue <- at:
	case <-sender.context.Done():
	}
}

func (sender *sender) SetInteractiveMode(guid string) {
	sender.interactiveRWM.Lock()
	defer sender.interactiveRWM.Unlock()
	sender.interactive[guid] = true
}

func (sender *sender) isInInteractiveMode(guid string) bool {
	sender.interactiveRWM.RLock()
	defer sender.interactiveRWM.RUnlock()
	return sender.interactive[guid]
}

func (sender *sender) GetClients() map[string]*client {
	sender.clientsRWM.RLock()
	defer sender.clientsRWM.RUnlock()
	clients := make(map[string]*client, len(sender.clients))
	for key, client := range sender.clients {
		clients[key] = client
	}
	return clients
}

func (sender *sender) Close() {
	atomic.StoreInt32(&sender.closing, 1)
	sender.cancel()
	sender.wg.Wait()
	for {
		// disconnect all sender client
		for _, client := range sender.GetClients() {
			client.Close()
		}
		// wait close
		time.Sleep(10 * time.Millisecond)
		if len(sender.GetClients()) == 0 {
			break
		}
	}
	sender.guid.Close()
}

func (sender *sender) logf(l logger.Level, format string, log ...interface{}) {
	sender.ctx.logger.Printf(l, "sender", format, log...)
}

func (sender *sender) log(l logger.Level, log ...interface{}) {
	sender.ctx.logger.Print(l, "sender", log...)
}

func (sender *sender) broadcast(guid, message []byte) (
	resp []*protocol.BroadcastResponse, success int) {
	clients := sender.GetClients()
	l := len(clients)
	if l == 0 {
		return
	}
	// broadcast parallel
	respChan := make(chan *protocol.BroadcastResponse, l)
	for _, c := range clients {
		go func(c *client) {
			respChan <- c.Broadcast(guid, message)
		}(c)
	}
	resp = make([]*protocol.BroadcastResponse, l)
	for i := 0; i < l; i++ {
		resp[i] = <-respChan
		if resp[i].Err == nil {
			success += 1
		}
	}
	return
}

func (sender *sender) send(role protocol.Role, guid, message []byte) (
	resp []*protocol.SendResponse, success int) {
	clients := sender.GetClients()
	l := len(clients)
	if l == 0 {
		return
	}
	// send parallel
	respChan := make(chan *protocol.SendResponse, l)
	switch role {
	case protocol.Node:
		for _, c := range clients {
			go func(c *client) {
				respChan <- c.SendToNode(guid, message)
			}(c)
		}
	case protocol.Beacon:
		for _, c := range clients {
			go func(c *client) {
				respChan <- c.SendToBeacon(guid, message)
			}(c)
		}
	default:
		panic("invalid Role")
	}
	resp = make([]*protocol.SendResponse, l)
	for i := 0; i < l; i++ {
		resp[i] = <-respChan
		if resp[i].Err == nil {
			success += 1
		}
	}
	return
}

func (sender *sender) acknowledge(role protocol.Role, guid, message []byte) (
	resp []*protocol.AcknowledgeResponse, success int) {
	clients := sender.GetClients()
	l := len(clients)
	if l == 0 {
		return
	}
	// acknowledge parallel
	respChan := make(chan *protocol.AcknowledgeResponse, l)
	switch role {
	case protocol.Node:
		for _, c := range clients {
			go func(c *client) {
				respChan <- c.AcknowledgeToNode(guid, message)
			}(c)
		}
	case protocol.Beacon:
		for _, c := range clients {
			go func(c *client) {
				respChan <- c.AcknowledgeToBeacon(guid, message)
			}(c)
		}
	default:
		panic("invalid Role")
	}
	resp = make([]*protocol.AcknowledgeResponse, l)
	for i := 0; i < l; i++ {
		resp[i] = <-respChan
		if resp[i].Err == nil {
			success += 1
		}
	}
	return
}

type senderWorker struct {
	ctx *sender

	maxBufferSize int

	// task
	bt *broadcastTask
	st *sendTask
	at *acknowledgeTask

	// key
	node   *mNode
	beacon *mBeacon
	aesKey []byte
	aesIV  []byte

	// prepare task objects
	preB protocol.Broadcast
	preS protocol.Send
	preA protocol.Acknowledge

	// objects
	buffer  *bytes.Buffer
	msgpack *msgpack.Encoder
	base64  io.WriteCloser
	hash    hash.Hash

	// temp
	roleGUID string
	err      error
}

func (sw *senderWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.Work() panic:")
			sw.ctx.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			go sw.Work()
		} else {
			sw.ctx.wg.Done()
		}
	}()
	// init
	minBufferSize := guid.Size + 9
	sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
	sw.msgpack = msgpack.NewEncoder(sw.buffer)
	sw.base64 = base64.NewEncoder(base64.StdEncoding, sw.buffer)
	sw.hash = sha256.New()
	// start handle task
	for {
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		case sw.at = <-sw.ctx.acknowledgeTaskQueue:
			sw.handleAcknowledgeTask()
		case sw.st = <-sw.ctx.sendTaskQueue:
			sw.handleSendTask()
		case sw.bt = <-sw.ctx.broadcastTaskQueue:
			sw.handleBroadcastTask()
		case <-sw.ctx.context.Done():
			return
		}
	}
}

func (sw *senderWorker) handleAcknowledgeTask() {
	defer sw.ctx.acknowledgeTaskPool.Put(sw.at)
	sw.preA.GUID = sw.ctx.guid.Get()
	sw.preA.RoleGUID = sw.at.RoleGUID
	sw.preA.SendGUID = sw.at.SendGUID
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preA.GUID)
	sw.buffer.Write(sw.preA.RoleGUID)
	sw.buffer.Write(sw.preA.SendGUID)
	sw.preA.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack
	sw.buffer.Reset()
	sw.err = sw.msgpack.Encode(sw.preA)
	if sw.err != nil {
		panic(sw.err)
	}
	// TODO try, if failed
	sw.ctx.acknowledge(sw.at.Role, sw.preA.GUID, sw.buffer.Bytes())
}

func (sw *senderWorker) handleSendTask() {
	result := sw.ctx.sendResultPool.Get().(*protocol.SendResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleSendTask() panic:")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		sw.st.Result <- result
	}()
	// role GUID string
	sw.buffer.Reset()
	_, _ = sw.base64.Write(sw.st.GUID)
	_ = sw.base64.Close()
	sw.roleGUID = sw.buffer.String()
	// pack message(interface)
	if sw.st.MessageI != nil {
		sw.buffer.Reset()
		sw.buffer.Write(sw.st.Command)
		result.Err = sw.msgpack.Encode(sw.st.MessageI)
		if result.Err != nil {
			return
		}
		// don't worry copy, because encrypt
		sw.st.Message = sw.buffer.Bytes()
	}
	// check message size
	if len(sw.st.Message) > protocol.MaxMsgSize {
		result.Err = protocol.ErrTooBigMsg
		return
	}
	// set key
	switch sw.st.Role {
	case protocol.Beacon:
		sw.beacon, result.Err = sw.ctx.ctx.db.SelectBeacon(sw.st.GUID)
		if result.Err != nil {
			return
		}
		sw.aesKey = sw.beacon.SessionKey
		sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	case protocol.Node:
		sw.node, result.Err = sw.ctx.ctx.db.SelectNode(sw.st.GUID)
		if result.Err != nil {
			return
		}
		sw.aesKey = sw.node.SessionKey
		sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	default:
		panic("invalid st.Role")
	}
	// encrypt
	sw.preS.Message, result.Err = aes.CBCEncrypt(sw.st.Message, sw.aesKey, sw.aesIV)
	if result.Err != nil {
		return
	}
	// set GUID
	sw.preS.GUID = sw.ctx.guid.Get()
	sw.preS.RoleGUID = sw.st.GUID
	// hash
	sw.hash.Reset()
	sw.hash.Write(sw.st.Message)
	sw.preS.Hash = sw.hash.Sum(nil)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preS.GUID)
	sw.buffer.Write(sw.preS.RoleGUID)
	sw.buffer.Write(sw.preS.Message)
	sw.buffer.Write(sw.preS.Hash)
	sw.preS.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack
	sw.buffer.Reset()
	result.Err = sw.msgpack.Encode(sw.preS)
	if result.Err != nil {
		return
	}
	// TODO query
	// check is need to write message to the database
	if sw.st.Role == protocol.Beacon && !sw.ctx.isInInteractiveMode(sw.roleGUID) {
		result.Err = sw.ctx.ctx.db.InsertBeaconMessage(sw.st.GUID, sw.buffer.Bytes())
		if result.Err == nil {
			result.Success = 1
		}
		return
	}
	// send
	result.Responses, result.Success = sw.ctx.send(sw.st.Role, sw.preS.GUID, sw.buffer.Bytes())
	if result.Success == 0 {
		result.Err = ErrSendFailed
		return
	}
}

func (sw *senderWorker) handleBroadcastTask() {
	result := sw.ctx.broadcastResultPool.Get().(*protocol.BroadcastResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleBroadcastTask() panic:")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		sw.bt.Result <- result
	}()
	// pack message(interface)
	if sw.bt.MessageI != nil {
		sw.buffer.Reset()
		sw.buffer.Write(sw.bt.Command)
		result.Err = sw.msgpack.Encode(sw.bt.MessageI)
		if result.Err != nil {
			return
		}
		// don't worry copy, because encrypt
		sw.bt.Message = sw.buffer.Bytes()
	}
	// check message size
	if len(sw.bt.Message) > protocol.MaxMsgSize {
		result.Err = protocol.ErrTooBigMsg
		return
	}
	// encrypt
	sw.preB.Message, result.Err = sw.ctx.ctx.global.Encrypt(sw.bt.Message)
	if result.Err != nil {
		return
	}
	// GUID
	sw.preB.GUID = sw.ctx.guid.Get()
	// hash
	sw.hash.Reset()
	sw.hash.Write(sw.bt.Message)
	sw.preB.Hash = sw.hash.Sum(nil)
	// sign
	sw.buffer.Reset()
	sw.buffer.Write(sw.preB.GUID)
	sw.buffer.Write(sw.preB.Message)
	sw.buffer.Write(sw.preB.Hash)
	sw.preB.Signature = sw.ctx.ctx.global.Sign(sw.buffer.Bytes())
	// pack
	sw.buffer.Reset()
	result.Err = sw.msgpack.Encode(sw.preB)
	if result.Err != nil {
		return
	}
	result.Responses, result.Success = sw.ctx.broadcast(sw.preB.GUID, sw.buffer.Bytes())
	if result.Success == 0 {
		result.Err = ErrBroadcastFailed
	}
}

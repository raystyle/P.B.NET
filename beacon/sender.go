package beacon

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"hash"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/convert"
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
	ErrFailedToQuery  = fmt.Errorf("failed to query")
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

// queryTask is used to query message from the Controller.
type queryTask struct {
	Index  uint64
	Result chan<- *protocol.QueryResult
}

// sender is used to send message to Controller, it can connect other Nodes.
type sender struct {
	ctx *Beacon

	maxConns atomic.Value

	sendTaskQueue  chan *sendTask
	ackTaskQueue   chan *ackTask
	queryTaskQueue chan *queryTask

	sendTaskPool  sync.Pool
	ackTaskPool   sync.Pool
	queryTaskPool sync.Pool

	sendDonePool  sync.Pool
	ackDonePool   sync.Pool
	queryDonePool sync.Pool

	sendResultPool  sync.Pool
	ackResultPool   sync.Pool
	queryResultPool sync.Pool

	deflateWriterPool sync.Pool
	hmacPool          sync.Pool

	// key = Node GUID
	clients    map[guid.GUID]*Client
	clientsRWM sync.RWMutex

	// wait Controller acknowledge
	ackSlots    map[guid.GUID]chan struct{}
	ackSlotsRWM sync.RWMutex
	ackSlotPool sync.Pool

	// query mode
	index    uint64
	indexRWM sync.RWMutex

	guid *guid.Generator

	inClose int32
	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newSender(ctx *Beacon, config *Config) (*sender, error) {
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
		ctx:            ctx,
		sendTaskQueue:  make(chan *sendTask, cfg.QueueSize),
		ackTaskQueue:   make(chan *ackTask, cfg.QueueSize),
		queryTaskQueue: make(chan *queryTask, cfg.QueueSize),
		clients:        make(map[guid.GUID]*Client),
		ackSlots:       make(map[guid.GUID]chan struct{}),
	}
	sender.context, sender.cancel = context.WithCancel(context.Background())

	maxConns := cfg.MaxConns
	sender.maxConns.Store(maxConns)

	sender.sendTaskPool.New = func() interface{} {
		return new(sendTask)
	}
	sender.ackTaskPool.New = func() interface{} {
		return new(ackTask)
	}
	sender.queryTaskPool.New = func() interface{} {
		return new(queryTask)
	}

	sender.sendDonePool.New = func() interface{} {
		return make(chan *protocol.SendResult, 1)
	}
	sender.ackDonePool.New = func() interface{} {
		return make(chan *protocol.AcknowledgeResult, 1)
	}
	sender.queryDonePool.New = func() interface{} {
		return make(chan *protocol.QueryResult, 1)
	}

	sender.sendResultPool.New = func() interface{} {
		return new(protocol.SendResult)
	}
	sender.ackResultPool.New = func() interface{} {
		return new(protocol.AcknowledgeResult)
	}
	sender.queryResultPool.New = func() interface{} {
		return new(protocol.QueryResult)
	}

	sender.deflateWriterPool.New = func() interface{} {
		writer, _ := flate.NewWriter(nil, flate.BestCompression)
		return writer
	}
	sessionKey := ctx.global.SessionKey()
	sender.hmacPool.New = func() interface{} {
		key := sessionKey.Get()
		defer sessionKey.Put(key)
		return hmac.New(sha256.New, key)
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
		return errors.WithMessage(err, "failed to create client")
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
	case <-ctx.Done():
		return ctx.Err()
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

// Acknowledge is used to acknowledge Controller that Beacon has received this message
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

// HandleAcknowledge is used to notice the Beacon that the Controller
// has received the send message.
func (sender *sender) HandleAcknowledge(send *guid.GUID) {
	sender.ackSlotsRWM.RLock()
	defer sender.ackSlotsRWM.RUnlock()
	if ch, ok := sender.ackSlots[*send]; ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Query is used to query message from the Controller.
func (sender *sender) Query() error {
	if sender.isClosed() {
		return ErrSenderClosed
	}
	done := sender.queryDonePool.Get().(chan *protocol.QueryResult)
	defer sender.queryDonePool.Put(done)
	qt := sender.queryTaskPool.Get().(*queryTask)
	defer sender.queryTaskPool.Put(qt)
	qt.Index = sender.getQueryIndex()
	qt.Result = done
	// send to task queue
	select {
	case sender.queryTaskQueue <- qt:
	case <-sender.context.Done():
		return ErrSenderClosed
	}
	result := <-done
	defer sender.queryResultPool.Put(result)
	return result.Err
}

func (sender *sender) getQueryIndex() uint64 {
	sender.indexRWM.RLock()
	defer sender.indexRWM.RUnlock()
	return sender.index
}

// AddQueryIndex is used to add query index.
func (sender *sender) AddQueryIndex(index uint64) bool {
	sender.indexRWM.Lock()
	defer sender.indexRWM.Unlock()
	if index != sender.index {
		return false
	}
	sender.index++
	return true
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

func (sender *sender) send(guid *guid.GUID, data *bytes.Buffer) ([]*protocol.SendResponse, int) {
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
					b := xpanic.Print(r, "sender.send")
					sender.log(logger.Fatal, b)
				}
			}()
			response <- c.Send(guid, data)
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

func (sender *sender) acknowledge(
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
					b := xpanic.Print(r, "sender.acknowledge")
					sender.log(logger.Fatal, b)
				}
			}()
			response <- c.Acknowledge(guid, data)
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

func (sender *sender) query(
	guid *guid.GUID,
	data *bytes.Buffer,
) ([]*protocol.QueryResponse, int) {
	clients := sender.Clients()
	l := len(clients)
	if l == 0 {
		return nil, 0
	}
	// acknowledge parallel
	response := make(chan *protocol.QueryResponse, l)
	for _, client := range clients {
		go func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					b := xpanic.Print(r, "sender.query")
					sender.log(logger.Fatal, b)
				}
			}()
			response <- c.Query(guid, data)
		}(client)
	}
	var success int
	responses := make([]*protocol.QueryResponse, l)
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

	// prepare task objects
	preS protocol.Send
	preA protocol.Acknowledge
	preQ protocol.Query

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
	beaconGUID := *sw.ctx.ctx.global.GUID()
	sw.preS.RoleGUID = beaconGUID
	sw.preA.RoleGUID = beaconGUID
	sw.preQ.BeaconGUID = beaconGUID
	// must stop at once, or maybe timeout at the first time.
	sw.timer = time.NewTimer(time.Minute)
	sw.timer.Stop()
	defer sw.timer.Stop()
	var (
		st *sendTask
		at *ackTask
		qt *queryTask
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
		case qt = <-sw.ctx.queryTaskQueue:
			sw.handleQueryTask(qt)
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
	beaconGUID := *sw.ctx.ctx.global.GUID()
	sw.preA.RoleGUID = beaconGUID
	sw.preQ.BeaconGUID = beaconGUID
	var (
		at *ackTask
		qt *queryTask
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
		case qt = <-sw.ctx.queryTaskQueue:
			sw.handleQueryTask(qt)
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
	result.Responses, result.Success = sw.ctx.send(&sw.preS.GUID, sw.buffer)
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
	// HMAC
	sw.calculateSendHMAC()
	// self validate
	result.Err = sw.preS.Validate()
	if result.Err != nil {
		panic("sender handleSendTask error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preS.Pack(sw.buffer)
}

func (sw *senderWorker) calculateSendHMAC() {
	h := sw.ctx.hmacPool.Get().(hash.Hash)
	defer sw.ctx.hmacPool.Put(h)
	h.Reset()
	h.Write(sw.preS.GUID[:])
	h.Write(sw.preS.RoleGUID[:])
	h.Write([]byte{sw.preS.Deflate})
	h.Write(sw.preS.Message)
	sw.preS.Hash = h.Sum(nil)
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
	// set GUID
	sw.preA.GUID = *sw.ctx.guid.Get()
	sw.preA.SendGUID = *at.SendGUID
	// HMAC
	sw.calculateAcknowledgeHMAC()
	// self validate
	result.Err = sw.preA.Validate()
	if result.Err != nil {
		panic("sender handleAcknowledgeTask error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preA.Pack(sw.buffer)
	// acknowledge
	result.Responses, result.Success = sw.ctx.acknowledge(&sw.preA.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToAck
	}
}

func (sw *senderWorker) calculateAcknowledgeHMAC() {
	h := sw.ctx.hmacPool.Get().(hash.Hash)
	defer sw.ctx.hmacPool.Put(h)
	h.Reset()
	h.Write(sw.preA.GUID[:])
	h.Write(sw.preA.RoleGUID[:])
	h.Write(sw.preA.SendGUID[:])
	sw.preA.Hash = h.Sum(nil)
}

func (sw *senderWorker) handleQueryTask(qt *queryTask) {
	result := sw.ctx.queryResultPool.Get().(*protocol.QueryResult)
	result.Clean()
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "senderWorker.handleQueryTask")
			sw.ctx.log(logger.Fatal, err)
			result.Err = err
		}
		qt.Result <- result
	}()
	sw.preQ.GUID = *sw.ctx.guid.Get()
	sw.preQ.Index = qt.Index
	// HMAC
	sw.calculateQueryHMAC()
	// self validate
	result.Err = sw.preQ.Validate()
	if result.Err != nil {
		panic("sender handleQueryTask error: " + result.Err.Error())
	}
	// pack
	sw.buffer.Reset()
	sw.preQ.Pack(sw.buffer)
	// query
	result.Responses, result.Success = sw.ctx.query(&sw.preQ.GUID, sw.buffer)
	if len(result.Responses) == 0 {
		result.Err = ErrNoConnections
		return
	}
	if result.Success == 0 {
		result.Err = ErrFailedToQuery
	}
}

func (sw *senderWorker) calculateQueryHMAC() {
	h := sw.ctx.hmacPool.Get().(hash.Hash)
	defer sw.ctx.hmacPool.Put(h)
	h.Reset()
	h.Write(sw.preQ.GUID[:])
	h.Write(sw.preQ.BeaconGUID[:])
	h.Write(convert.Uint64ToBytes(sw.preQ.Index))
	sw.preQ.Hash = h.Sum(nil)
}

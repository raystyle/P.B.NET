package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type syncer struct {
	ctx         *CTRL
	guidTimeout float64
	maxClient   atomic.Value

	// key = base64(GUID) value = timestamp
	nodeSendGUID       map[string]int64
	nodeSendGUIDRWM    sync.RWMutex
	beaconSendGUID     map[string]int64
	beaconSendGUIDRWM  sync.RWMutex
	beaconQueryGUID    map[string]int64
	beaconQueryGUIDRWM sync.RWMutex

	nodeSendQueue    chan *protocol.Send
	beaconSendQueue  chan *protocol.Send
	beaconQueryQueue chan *protocol.Query

	// connected node key=base64(guid)
	sClients    map[string]*sClient
	sClientsRWM sync.RWMutex

	inClose    int32
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSyncer(ctx *CTRL, cfg *Config) (*syncer, error) {
	// check config
	if cfg.MaxBufferSize < 4096 {
		return nil, errors.New("max buffer size < 4096")
	}
	if cfg.MaxSyncerClient < 1 {
		return nil, errors.New("max syncer < 1")
	}
	if cfg.SyncerWorker < 2 {
		return nil, errors.New("worker number < 2")
	}
	if cfg.SyncerQueueSize < 512 {
		return nil, errors.New("worker task queue size < 512")
	}
	syncer := syncer{
		ctx:              ctx,
		guidTimeout:      float64(cfg.MessageTimeout),
		nodeSendGUID:     make(map[string]int64, cfg.SyncerQueueSize),
		beaconSendGUID:   make(map[string]int64, cfg.SyncerQueueSize),
		beaconQueryGUID:  make(map[string]int64, cfg.SyncerQueueSize),
		nodeSendQueue:    make(chan *protocol.Send, cfg.SyncerQueueSize),
		beaconSendQueue:  make(chan *protocol.Send, cfg.SyncerQueueSize),
		beaconQueryQueue: make(chan *protocol.Query, cfg.SyncerQueueSize),
		sClients:         make(map[string]*sClient, cfg.MaxSyncerClient),
		stopSignal:       make(chan struct{}),
	}
	syncer.maxClient.Store(cfg.MaxSyncerClient)
	// start workers
	for i := 0; i < cfg.SyncerWorker; i++ {
		worker := syncerWorker{
			ctx:           &syncer,
			maxBufferSize: cfg.MaxBufferSize,
		}
		syncer.wg.Add(1)
		go worker.Work()
	}
	syncer.wg.Add(1)
	go syncer.guidCleaner()
	syncer.wg.Add(1)
	go syncer.watcher()
	return &syncer, nil
}

// Connect is used to connect node for sync message
func (syncer *syncer) Connect(node *bootstrap.Node, guid []byte) error {
	if syncer.isClosed() {
		return errors.New("syncer is closed")
	}
	syncer.sClientsRWM.Lock()
	defer syncer.sClientsRWM.Unlock()
	if len(syncer.sClients) >= syncer.getMaxClient() {
		return errors.New("connected node number > max syncer")
	}
	cfg := clientCfg{
		Node:     node,
		NodeGUID: guid,
	}
	sClient, err := newSClient(syncer, &cfg)
	if err != nil {
		return errors.WithMessage(err, "connect node failed")
	}
	key := base64.StdEncoding.EncodeToString(guid)
	syncer.sClients[key] = sClient
	syncer.logf(logger.Info, "connect node %s", node.Address)
	return nil
}

func (syncer *syncer) Disconnect(guid string) error {
	syncer.sClientsRWM.RLock()
	if sClient, ok := syncer.sClients[guid]; ok {
		syncer.sClientsRWM.RUnlock()
		sClient.Close()
		syncer.logf(logger.Info, "disconnect node %s", sClient.Node.Address)
		return nil
	} else {
		syncer.sClientsRWM.RUnlock()
		return errors.Errorf("syncer client %s doesn't exist", guid)
	}
}

func (syncer *syncer) Clients() map[string]*sClient {
	syncer.sClientsRWM.RLock()
	l := len(syncer.sClients)
	if l == 0 {
		syncer.sClientsRWM.RUnlock()
		return nil
	}
	// copy map
	sClients := make(map[string]*sClient, l)
	for key, client := range syncer.sClients {
		sClients[key] = client
	}
	syncer.sClientsRWM.RUnlock()
	return sClients
}

func (syncer *syncer) isClosed() bool {
	return atomic.LoadInt32(&syncer.inClose) != 0
}

func (syncer *syncer) Close() {
	atomic.StoreInt32(&syncer.inClose, 1)
	// disconnect all syncer clients
	for key := range syncer.Clients() {
		_ = syncer.Disconnect(key)
	}
	// wait close
	for {
		time.Sleep(10 * time.Millisecond)
		if len(syncer.Clients()) == 0 {
			break
		}
	}
	close(syncer.stopSignal)
	syncer.wg.Wait()
}

func (syncer *syncer) logf(l logger.Level, format string, log ...interface{}) {
	syncer.ctx.Printf(l, "syncer", format, log...)
}

func (syncer *syncer) log(l logger.Level, log ...interface{}) {
	syncer.ctx.Print(l, "syncer", log...)
}

func (syncer *syncer) logln(l logger.Level, log ...interface{}) {
	syncer.ctx.Println(l, "syncer", log...)
}

// getMaxSyncerClient is used to get current max syncer client number
func (syncer *syncer) getMaxClient() int {
	return syncer.maxClient.Load().(int)
}

// watcher is used to check connect nodes number
// connected nodes number < syncer.maxClient, try to connect more node
func (syncer *syncer) watcher() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer watcher panic:", r)
			syncer.log(logger.Fatal, err)
			// restart watcher
			time.Sleep(time.Second)
			syncer.wg.Add(1)
			go syncer.watcher()
		}
		syncer.wg.Done()
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	isMax := func() bool {
		// get current syncer client number
		syncer.sClientsRWM.RLock()
		sClientsLen := len(syncer.sClients)
		syncer.sClientsRWM.RUnlock()
		return sClientsLen >= syncer.getMaxClient()
	}
	watch := func() {
		if isMax() {
			return
		}
		// select nodes
		// TODO watcher
	}
	for {
		select {
		case <-ticker.C:
			watch()
		case <-syncer.stopSignal:
			return
		}
	}
}

// task from syncer client
func (syncer *syncer) AddNodeSend(s *protocol.Send) {
	select {
	case syncer.nodeSendQueue <- s:
	case <-syncer.stopSignal:
	}
}

func (syncer *syncer) AddBeaconSend(s *protocol.Send) {
	select {
	case syncer.beaconSendQueue <- s:
	case <-syncer.stopSignal:
	}
}

func (syncer *syncer) AddBeaconQuery(q *protocol.Query) {
	select {
	case syncer.beaconQueryQueue <- q:
	case <-syncer.stopSignal:
	}
}

func (syncer *syncer) CheckGUIDTimestamp(guid []byte) (bool, int64) {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.guidTimeout {
		return false, 0
	}
	return true, timestamp
}

func (syncer *syncer) CheckNodeSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.nodeSendGUIDRWM.Lock()
		if _, ok := syncer.nodeSendGUID[key]; ok {
			syncer.nodeSendGUIDRWM.Unlock()
			return false
		} else {
			syncer.nodeSendGUID[key] = timestamp
			syncer.nodeSendGUIDRWM.Unlock()
			return true
		}
	} else {
		syncer.nodeSendGUIDRWM.RLock()
		_, ok := syncer.nodeSendGUID[key]
		syncer.nodeSendGUIDRWM.RUnlock()
		return !ok
	}
}

func (syncer *syncer) CheckBeaconSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.beaconSendGUIDRWM.Lock()
		if _, ok := syncer.beaconSendGUID[key]; ok {
			syncer.beaconSendGUIDRWM.Unlock()
			return false
		} else {
			syncer.beaconSendGUID[key] = timestamp
			syncer.beaconSendGUIDRWM.Unlock()
			return true
		}
	} else {
		syncer.beaconSendGUIDRWM.RLock()
		_, ok := syncer.beaconSendGUID[key]
		syncer.beaconSendGUIDRWM.RUnlock()
		return !ok
	}
}

func (syncer *syncer) CheckBeaconQueryGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.beaconQueryGUIDRWM.Lock()
		if _, ok := syncer.beaconQueryGUID[key]; ok {
			syncer.beaconQueryGUIDRWM.Unlock()
			return false
		} else {
			syncer.beaconQueryGUID[key] = timestamp
			syncer.beaconQueryGUIDRWM.Unlock()
			return true
		}
	} else {
		syncer.beaconQueryGUIDRWM.RLock()
		_, ok := syncer.beaconQueryGUID[key]
		syncer.beaconQueryGUIDRWM.RUnlock()
		return !ok
	}
}

// guidCleaner is use to clean expired guid
func (syncer *syncer) guidCleaner() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer guid cleaner panic:", r)
			syncer.log(logger.Fatal, err)
			// restart guid cleaner
			time.Sleep(time.Second)
			go syncer.guidCleaner()
		} else {
			syncer.wg.Done()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := syncer.ctx.global.Now().Unix()
			// clean node send
			syncer.nodeSendGUIDRWM.Lock()
			for key, timestamp := range syncer.nodeSendGUID {
				if float64(now-timestamp) > syncer.guidTimeout {
					delete(syncer.nodeSendGUID, key)
				}
			}
			syncer.nodeSendGUIDRWM.Unlock()
			// clean beacon send
			syncer.beaconSendGUIDRWM.Lock()
			for key, timestamp := range syncer.beaconSendGUID {
				if float64(now-timestamp) > syncer.guidTimeout {
					delete(syncer.beaconSendGUID, key)
				}
			}
			syncer.beaconSendGUIDRWM.Unlock()
			// clean beacon query
			syncer.beaconQueryGUIDRWM.Lock()
			for key, timestamp := range syncer.beaconQueryGUID {
				if float64(now-timestamp) > syncer.guidTimeout {
					delete(syncer.beaconQueryGUID, key)
				}
			}
			syncer.beaconQueryGUIDRWM.Unlock()
		case <-syncer.stopSignal:
			return
		}
	}
}

type syncerWorker struct {
	ctx           *syncer
	maxBufferSize int

	// task
	send  *protocol.Send
	query *protocol.Query

	// key
	node      *mNode
	beacon    *mBeacon
	publicKey ed25519.PublicKey
	aesKey    []byte
	aesIV     []byte

	buffer         *bytes.Buffer
	msgpackEncoder *msgpack.Encoder
	base64Encoder  io.WriteCloser
	hash           hash.Hash

	// temp
	nodeSyncer   *nodeSyncer
	beaconSyncer *beaconSyncer
	roleGUID     string
	roleSend     uint64
	ctrlReceive  uint64
	sub          uint64
	sClients     map[string]*sClient
	sClient      *sClient

	err error
}

func (sw *syncerWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncerWorker.Work() panic:", r)
			sw.ctx.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			go sw.Work()
		} else {
			sw.ctx.wg.Done()
		}
	}()
	// init buffer
	// protocol.SyncReceive buffer cap = guid.Size + 8 + 1 + guid.Size
	minBufferSize := 2*guid.Size + 9
	sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
	sw.msgpackEncoder = msgpack.NewEncoder(sw.buffer)
	sw.base64Encoder = base64.NewEncoder(base64.StdEncoding, sw.buffer)
	sw.hash = sha256.New()

	// start handle task
	for {
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		case sw.query = <-sw.ctx.beaconQueryQueue:
			sw.handleBeaconQuery()
		case sw.send = <-sw.ctx.beaconSendQueue:
			sw.handleBeaconSend()
		case sw.send = <-sw.ctx.nodeSendQueue:
			sw.handleNodeSend()
		case <-sw.ctx.stopSignal:
			return
		}
	}
}

func (sw *syncerWorker) handleBeaconQuery() {
	// set key
	sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.query.BeaconGUID)
	if sw.err != nil {
		sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.query.BeaconGUID, sw.err)
		return
	}
	sw.publicKey = sw.beacon.PublicKey
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.query.GUID)
	sw.buffer.Write(sw.query.BeaconGUID)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.query.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid query signature guid: %X", sw.query.BeaconGUID)
		return
	}
	// TODO broadcast message
}

func (sw *syncerWorker) handleBeaconSend() {
	// set key
	sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.send.RoleGUID)
	if sw.err != nil {
		sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.send.RoleGUID, sw.err)
		return
	}
	sw.publicKey = sw.beacon.PublicKey
	sw.aesKey = sw.beacon.SessionKey
	sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.send.GUID)
	sw.buffer.Write(sw.send.RoleGUID)
	sw.buffer.Write(sw.send.Message)
	sw.buffer.Write(sw.send.Hash)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.send.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid beacon send signature guid: %X", sw.send.RoleGUID)
		return
	}
	// decrypt
	sw.send.Message, sw.err = aes.CBCDecrypt(sw.send.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		sw.ctx.logf(logger.Exploit, "decrypt beacon %X send failed: %s", sw.send.RoleGUID, sw.err)
		return
	}
	// check hash
	sw.hash.Reset()
	sw.hash.Write(sw.send.Message)
	if !bytes.Equal(sw.hash.Sum(nil), sw.send.Hash) {
		sw.ctx.logf(logger.Exploit, "beacon %X send with wrong hash", sw.send.RoleGUID)
		return
	}
	sw.ctx.ctx.handler.HandleBeaconSend(sw.send)
	sw.ctx.ctx.sender.Acknowledge(protocol.Beacon, sw.send)
}

func (sw *syncerWorker) handleNodeSend() {
	// set key
	sw.node, sw.err = sw.ctx.ctx.db.SelectNode(sw.send.RoleGUID)
	if sw.err != nil {
		sw.ctx.logf(logger.Warning, "select node %X failed %s", sw.send.RoleGUID, sw.err)
		return
	}
	sw.publicKey = sw.node.PublicKey
	sw.aesKey = sw.node.SessionKey
	sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.send.GUID)
	sw.buffer.Write(sw.send.RoleGUID)
	sw.buffer.Write(sw.send.Message)
	sw.buffer.Write(sw.send.Hash)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.send.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid node send signature guid: %X", sw.send.RoleGUID)
		return
	}
	// decrypt
	sw.send.Message, sw.err = aes.CBCDecrypt(sw.send.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		sw.ctx.logf(logger.Exploit, "decrypt node %X send failed: %s", sw.send.RoleGUID, sw.err)
		return
	}
	// check hash
	sw.hash.Reset()
	sw.hash.Write(sw.send.Message)
	if !bytes.Equal(sw.hash.Sum(nil), sw.send.Hash) {
		sw.ctx.logf(logger.Exploit, "node %X send with wrong hash", sw.send.RoleGUID)
		return
	}
	sw.ctx.ctx.handler.HandleNodeSend(sw.send)
	sw.ctx.ctx.sender.Acknowledge(protocol.Node, sw.send)
}

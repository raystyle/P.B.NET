package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"io"
	"math"
	"sync"
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
	ctx              *CTRL
	workerQueueSize  int
	maxBlockWorker   int
	retryTimes       int
	retryInterval    time.Duration
	broadcastTimeout float64
	maxSyncerClient  int
	maxSyncerClientM sync.Mutex
	// -------------------handle broadcast------------------------
	// key=base64(guid) value=timestamp, check whether handled
	broadcastQueue   chan *protocol.Broadcast
	broadcastGUID    [2]map[string]int64
	broadcastGUIDRWM [2]sync.RWMutex
	// -----------------handle sync message-----------------------
	syncSendQueue      chan *protocol.SyncSend
	syncSendGUID       [2]map[string]int64
	syncSendGUIDRWM    [2]sync.RWMutex
	syncReceiveQueue   chan *protocol.SyncReceive
	syncReceiveGUID    [2]map[string]int64
	syncReceiveGUIDRWM [2]sync.RWMutex
	// -------------------handle sync task------------------------
	syncTaskQueue chan *protocol.SyncTask
	// check is sync
	syncStatus  [2]map[string]bool
	syncStatusM [2]sync.Mutex
	// prevent all workers handle sync task
	blockWorker  int
	blockWorkerM sync.Mutex

	// connected node key=base64(guid)
	sClients    map[string]*sClient
	sClientsRWM sync.RWMutex

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
	if cfg.ReserveWorker >= cfg.SyncerWorker {
		return nil, errors.New("reserve worker number >= worker number")
	}
	if cfg.RetryTimes < 3 {
		return nil, errors.New("retry time < 3")
	}
	if cfg.RetryInterval < 5*time.Second {
		return nil, errors.New("retry interval < 5s")
	}
	if cfg.BroadcastTimeout < 30*time.Second {
		return nil, errors.New("broadcast timeout < 30s")
	}
	syncer := syncer{
		ctx:              ctx,
		maxSyncerClient:  cfg.MaxSyncerClient,
		workerQueueSize:  cfg.SyncerQueueSize,
		maxBlockWorker:   cfg.SyncerWorker - cfg.ReserveWorker,
		retryTimes:       cfg.RetryTimes,
		retryInterval:    cfg.RetryInterval,
		broadcastTimeout: cfg.BroadcastTimeout.Seconds(),
		broadcastQueue:   make(chan *protocol.Broadcast, cfg.SyncerQueueSize),
		syncSendQueue:    make(chan *protocol.SyncSend, cfg.SyncerQueueSize),
		syncReceiveQueue: make(chan *protocol.SyncReceive, cfg.SyncerQueueSize),
		syncTaskQueue:    make(chan *protocol.SyncTask, cfg.SyncerQueueSize),
		sClients:         make(map[string]*sClient),
		stopSignal:       make(chan struct{}),
	}
	for i := 0; i < 2; i++ {
		syncer.broadcastGUID[i] = make(map[string]int64)
		syncer.syncSendGUID[i] = make(map[string]int64)
		syncer.syncReceiveGUID[i] = make(map[string]int64)
		syncer.syncStatus[i] = make(map[string]bool)
	}
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

func (syncer *syncer) Close() {
	close(syncer.stopSignal)
	syncer.wg.Wait()
}

// Connect is used to connect node for sync message
func (syncer *syncer) Connect(node *bootstrap.Node, guid []byte) error {
	syncer.sClientsRWM.Lock()
	defer syncer.sClientsRWM.Unlock()
	sClientsLen := len(syncer.sClients)
	if sClientsLen >= syncer.getMaxSyncerClient() {
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

func (syncer *syncer) logf(l logger.Level, format string, log ...interface{}) {
	syncer.ctx.Printf(l, "syncer", format, log...)
}

func (syncer *syncer) log(l logger.Level, log ...interface{}) {
	syncer.ctx.Print(l, "syncer", log...)
}

func (syncer *syncer) logln(l logger.Level, log ...interface{}) {
	syncer.ctx.Println(l, "syncer", log...)
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

// getMaxSyncerClient is used to get current max syncer client number
func (syncer *syncer) getMaxSyncerClient() int {
	syncer.maxSyncerClientM.Lock()
	maxSyncer := syncer.maxSyncerClient
	syncer.maxSyncerClientM.Unlock()
	return maxSyncer
}

// watcher is used to check connect nodes number
// connected nodes number < syncer.maxSyncerClient, try to connect more node
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
		return sClientsLen >= syncer.getMaxSyncerClient()
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
func (syncer *syncer) AddBroadcast(br *protocol.Broadcast) {
	if len(syncer.broadcastQueue) == syncer.workerQueueSize {
		go func() { // prevent block
			select {
			case syncer.broadcastQueue <- br:
			case <-syncer.stopSignal:
			}
		}()
	} else {
		select {
		case syncer.broadcastQueue <- br:
		case <-syncer.stopSignal:
		}
	}
}

// task from syncer client
func (syncer *syncer) AddSyncSend(ss *protocol.SyncSend) {
	if len(syncer.syncSendQueue) == syncer.workerQueueSize {
		go func() { // prevent block
			select {
			case syncer.syncSendQueue <- ss:
			case <-syncer.stopSignal:
			}
		}()
	} else {
		select {
		case syncer.syncSendQueue <- ss:
		case <-syncer.stopSignal:
		}
	}
}

// task from syncer client
func (syncer *syncer) AddSyncReceive(sr *protocol.SyncReceive) {
	if len(syncer.syncReceiveQueue) == syncer.workerQueueSize {
		go func() { // prevent block
			select {
			case syncer.syncReceiveQueue <- sr:
			case <-syncer.stopSignal:
			}
		}()
	} else {
		select {
		case syncer.syncReceiveQueue <- sr:
		case <-syncer.stopSignal:
		}
	}
}

// addSyncTask is used to
// worker use it
func (syncer *syncer) addSyncTask(task *protocol.SyncTask) {
	if len(syncer.syncTaskQueue) == syncer.workerQueueSize {
		go func() { // prevent block
			select {
			case syncer.syncTaskQueue <- task:
			case <-syncer.stopSignal:
			}
		}()
	} else {
		select {
		case syncer.syncTaskQueue <- task:
		case <-syncer.stopSignal:
		}
	}
}

const (
	syncerNode   = 0
	syncerBeacon = 1
)

// check xxx Token is used to check xxx is been handled
// xxx = broadcast, sync send, sync receive
// just tell others, but they can still send it by force

func (syncer *syncer) checkBroadcastToken(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.broadcastGUIDRWM[i].RLock()
	_, ok := syncer.broadcastGUID[i][key]
	syncer.broadcastGUIDRWM[i].RUnlock()
	return !ok
}

func (syncer *syncer) checkSyncSendToken(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncSendGUIDRWM[i].RLock()
	_, ok := syncer.syncSendGUID[i][key]
	syncer.syncSendGUIDRWM[i].RUnlock()
	return !ok
}

func (syncer *syncer) checkSyncReceiveToken(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncReceiveGUIDRWM[i].RLock()
	_, ok := syncer.syncReceiveGUID[i][key]
	syncer.syncReceiveGUIDRWM[i].RUnlock()
	return !ok
}

// check xxx GUID is used to check xxx is been handled
// prevent others send same message
// xxx = broadcast, sync send, sync receive
// must use Abs to prevent future timestamp

func (syncer *syncer) checkBroadcastGUID(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.broadcastGUIDRWM[i].Lock()
	if _, ok := syncer.broadcastGUID[i][key]; !ok {
		syncer.broadcastGUID[i][key] = timestamp
		syncer.broadcastGUIDRWM[i].Unlock()
		return true
	} else {
		syncer.broadcastGUIDRWM[i].Unlock()
		return false
	}
}

func (syncer *syncer) checkSyncSendGUID(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncSendGUIDRWM[i].Lock()
	if _, ok := syncer.syncSendGUID[i][key]; !ok {
		syncer.syncSendGUID[i][key] = timestamp
		syncer.syncSendGUIDRWM[i].Unlock()
		return true
	} else {
		syncer.syncSendGUIDRWM[i].Unlock()
		return false
	}
}

func (syncer *syncer) checkSyncReceiveGUID(role protocol.Role, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncReceiveGUIDRWM[i].Lock()
	if _, ok := syncer.syncReceiveGUID[i][key]; !ok {
		syncer.syncReceiveGUID[i][key] = timestamp
		syncer.syncReceiveGUIDRWM[i].Unlock()
		return true
	} else {
		syncer.syncReceiveGUIDRWM[i].Unlock()
		return false
	}
}

// guidCleaner is use to clean expire guid
func (syncer *syncer) guidCleaner() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer guid cleaner panic:", r)
			syncer.log(logger.Fatal, err)
			// restart guid cleaner
			time.Sleep(time.Second)
			syncer.wg.Add(1)
			go syncer.guidCleaner()
		}
		syncer.wg.Done()
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := syncer.ctx.global.Now().Unix()
			for i := 0; i < 2; i++ {
				// clean broadcast
				syncer.broadcastGUIDRWM[i].Lock()
				for key, timestamp := range syncer.broadcastGUID[i] {
					if float64(now-timestamp) > syncer.broadcastTimeout {
						delete(syncer.broadcastGUID[i], key)
					}
				}
				syncer.broadcastGUIDRWM[i].Unlock()
				// clean sync send
				syncer.syncSendGUIDRWM[i].Lock()
				for key, timestamp := range syncer.syncSendGUID[i] {
					if float64(now-timestamp) > syncer.broadcastTimeout {
						delete(syncer.syncSendGUID[i], key)
					}
				}
				syncer.syncSendGUIDRWM[i].Unlock()
				// clean sync receive
				syncer.syncReceiveGUIDRWM[i].Lock()
				for key, timestamp := range syncer.syncReceiveGUID[i] {
					if float64(now-timestamp) > syncer.broadcastTimeout {
						delete(syncer.syncReceiveGUID[i], key)
					}
				}
				syncer.syncReceiveGUIDRWM[i].Unlock()
			}
		case <-syncer.stopSignal:
			return
		}
	}
}

// DeleteSyncStatus is used to delete syncStatus
// if delete role, must delete it
func (syncer *syncer) DeleteSyncStatus(role protocol.Role, guid string) {
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncStatusM[i].Lock()
	delete(syncer.syncStatus[i], guid)
	syncer.syncStatusM[i].Unlock()
}

// isSync is used to check role is synchronizing
// if not set flag and lock it
func (syncer *syncer) isSync(role protocol.Role, guid string) bool {
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncStatusM[i].Lock()
	if syncer.syncStatus[i][guid] {
		syncer.syncStatusM[i].Unlock()
		return true
	} else {
		syncer.syncStatus[i][guid] = true
		syncer.syncStatusM[i].Unlock()
		return false
	}
}

// syncDone is used to set sync done
func (syncer *syncer) syncDone(role protocol.Role, guid string) {
	i := 0
	switch role {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		panic("invalid role")
	}
	syncer.syncStatusM[i].Lock()
	syncer.syncStatus[i][guid] = false
	syncer.syncStatusM[i].Unlock()
}

// if all workers handle sync task, syncer will dead

// isBlock is used to check reserve worker number
func (syncer *syncer) isBlock() bool {
	// check block worker number
	syncer.blockWorkerM.Lock()
	if syncer.blockWorker >= syncer.maxBlockWorker {
		syncer.blockWorkerM.Unlock()
		return true
	} else {
		syncer.blockWorker += 1
		syncer.blockWorkerM.Unlock()
		return false
	}
}

// blockDone is used to delete block worker
func (syncer *syncer) blockDone() {
	syncer.blockWorkerM.Lock()
	syncer.blockWorker -= 1
	syncer.blockWorkerM.Unlock()
}

type syncerWorker struct {
	ctx           *syncer
	maxBufferSize int

	// task
	b  *protocol.Broadcast
	ss *protocol.SyncSend
	sr *protocol.SyncReceive
	st *protocol.SyncTask

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
	nodeSyncer     *nodeSyncer
	beaconSyncer   *beaconSyncer
	roleGUID       string
	roleSend       uint64
	ctrlReceive    uint64
	sub            uint64
	sClients       map[string]*sClient
	sClient        *sClient
	syncQuery      *protocol.SyncQuery
	syncReply      *protocol.SyncReply
	syncQueryBytes []byte
	err            error
}

func (sw *syncerWorker) handleSyncReceive() {
	// check role and set key
	switch sw.sr.Role {
	case protocol.Beacon:
		sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.sr.RoleGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.sr.RoleGUID, sw.err)
			return
		}
		sw.publicKey = sw.beacon.PublicKey
	case protocol.Node:
		sw.node, sw.err = sw.ctx.ctx.db.SelectNode(sw.sr.RoleGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select node %X failed %s", sw.sr.RoleGUID, sw.err)
			return
		}
		sw.publicKey = sw.node.PublicKey
	default:
		panic("invalid sr.Role")
	}
	// must verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.sr.GUID)
	sw.buffer.Write(convert.Uint64ToBytes(sw.sr.Height))
	sw.buffer.WriteByte(sw.sr.Role.Byte())
	sw.buffer.Write(sw.sr.RoleGUID)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.sr.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid sync receive signature %s guid: %X",
			sw.sr.Role, sw.sr.RoleGUID)
		return
	}
	if !sw.ctx.checkSyncReceiveGUID(sw.sr.Role, sw.sr.GUID) {
		return
	}
	sw.sr.Height += 1
	// update role receive
	switch sw.sr.Role {
	case protocol.Beacon:
		sw.err = sw.ctx.ctx.db.UpdateBSBeaconReceive(sw.sr.RoleGUID, sw.sr.Height)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "update %X beacon receive failed %s", sw.sr.RoleGUID, sw.err)
		}
	case protocol.Node:
		sw.err = sw.ctx.ctx.db.UpdateNSNodeReceive(sw.sr.RoleGUID, sw.sr.Height)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "update %X node receive failed %s", sw.sr.RoleGUID, sw.err)
		}
	default:
		panic("invalid sr.Role")
	}
}

func (sw *syncerWorker) handleSyncSend() {
	// set key
	switch sw.ss.SenderRole {
	case protocol.Beacon:
		sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.ss.SenderGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.ss.SenderGUID, sw.err)
			return
		}
		sw.publicKey = sw.beacon.PublicKey
		sw.aesKey = sw.beacon.SessionKey
		sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	case protocol.Node:
		sw.node, sw.err = sw.ctx.ctx.db.SelectNode(sw.ss.SenderGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select node %X failed %s", sw.ss.SenderGUID, sw.err)
			return
		}
		sw.publicKey = sw.node.PublicKey
		sw.aesKey = sw.node.SessionKey
		sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	default:
		panic("invalid ss.SenderRole")
	}
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.ss.GUID)
	sw.buffer.Write(convert.Uint64ToBytes(sw.ss.Height))
	sw.buffer.Write(sw.ss.Message)
	sw.buffer.Write(sw.ss.Hash)
	sw.buffer.WriteByte(sw.ss.SenderRole.Byte())
	sw.buffer.Write(sw.ss.SenderGUID)
	sw.buffer.WriteByte(sw.ss.ReceiverRole.Byte())
	sw.buffer.Write(sw.ss.ReceiverGUID)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.ss.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid sync send signature %s guid: %X",
			sw.ss.SenderRole, sw.ss.SenderGUID)
		return
	}
	// check handled
	if !sw.ctx.checkSyncSendGUID(sw.ss.SenderRole, sw.ss.GUID) {
		return
	}
	sw.ss.Height += 1 // index -> height
	// update role send
	switch sw.ss.SenderRole {
	case protocol.Beacon:
		sw.err = sw.ctx.ctx.db.UpdateBSBeaconSend(sw.ss.SenderGUID, sw.ss.Height)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "update %X beacon send failed %s", sw.ss.SenderGUID, sw.err)
			return
		}
	case protocol.Node:
		sw.err = sw.ctx.ctx.db.UpdateNSNodeSend(sw.ss.SenderGUID, sw.ss.Height)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "update %X node send failed %s", sw.ss.SenderGUID, sw.err)
			return
		}
	default:
		panic("invalid ss.SenderRole")
	}
	// lock role
	sw.buffer.Reset()
	_, _ = sw.base64Encoder.Write(sw.ss.SenderGUID)
	_ = sw.base64Encoder.Close()
	sw.roleGUID = sw.buffer.String()
	// check sync
	if sw.ctx.isSync(sw.ss.SenderRole, sw.roleGUID) {
		return
	}
	defer sw.ctx.syncDone(sw.ss.SenderRole, sw.roleGUID)
	// select role send & controller receive
	// must select again, because maybe update
	// role send at the same time
	switch sw.ss.SenderRole {
	case protocol.Beacon:
		sw.beaconSyncer, sw.err = sw.ctx.ctx.db.SelectBeaconSyncer(sw.ss.SenderGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select beacon syncer %X failed %s", sw.ss.SenderGUID, sw.err)
			return
		}
		sw.beaconSyncer.RLock()
		sw.roleSend = sw.beaconSyncer.BeaconSend
		sw.ctrlReceive = sw.beaconSyncer.CtrlRecv
		sw.beaconSyncer.RUnlock()
	case protocol.Node:
		sw.nodeSyncer, sw.err = sw.ctx.ctx.db.SelectNodeSyncer(sw.ss.SenderGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select node syncer %X failed %s", sw.ss.SenderGUID, sw.err)
			return
		}
		sw.nodeSyncer.RLock()
		sw.roleSend = sw.nodeSyncer.NodeSend
		sw.ctrlReceive = sw.nodeSyncer.CtrlRecv
		sw.nodeSyncer.RUnlock()
	default:
		panic("invalid ss.SenderRole")
	}
	// check height
	sw.sub = sw.roleSend - sw.ctrlReceive
	switch {
	// case sw.sub < 1: // receive handled message
	case sw.sub == 1: // only one new message, handle it
		// decrypt
		sw.ss.Message, sw.err = aes.CBCDecrypt(sw.ss.Message, sw.aesKey, sw.aesIV)
		if sw.err != nil {
			sw.ctx.logf(logger.Exploit, "decrypt %s guid: %X sync send failed: %s",
				sw.ss.SenderRole, sw.ss.SenderGUID, sw.err)
			return
		}
		// check hash
		sw.hash.Reset()
		sw.hash.Write(sw.ss.Message)
		if !bytes.Equal(sw.hash.Sum(nil), sw.ss.Hash) {
			sw.ctx.logf(logger.Exploit, "%s guid: %X sync send with wrong hash",
				sw.ss.SenderRole, sw.ss.SenderGUID)
			return
		}
		sw.ctx.ctx.handleMessage(sw.ss.Message, sw.ss.SenderRole, sw.ss.SenderGUID, sw.roleSend-1)
		// update controller receive
		switch sw.ss.SenderRole {
		case protocol.Beacon:
			sw.err = sw.ctx.ctx.db.UpdateBSCtrlReceive(sw.ss.SenderGUID, sw.roleSend)
			if sw.err != nil {
				sw.ctx.logf(logger.Warning, "update beacon syncer %X ctrl send failed %s",
					sw.ss.SenderGUID, sw.err)
				return
			}
		case protocol.Node:
			sw.err = sw.ctx.ctx.db.UpdateNSCtrlReceive(sw.ss.SenderGUID, sw.roleSend)
			if sw.err != nil {
				sw.ctx.logf(logger.Warning, "update node syncer %X ctrl send failed %s",
					sw.ss.SenderGUID, sw.err)
				return
			}
		default:
			panic("invalid ss.SenderRole")
		}
		// notice node to delete message
		sw.ctx.ctx.sender.SyncReceive(sw.ss.SenderRole, sw.ss.SenderGUID, sw.roleSend-1)
	case sw.sub > 1: // get old message and need sync more message
		sw.ctx.addSyncTask(&protocol.SyncTask{
			Role: sw.ss.SenderRole,
			GUID: sw.ss.SenderGUID,
		})
	}
}

func (sw *syncerWorker) handleBroadcast() {
	// set key
	switch sw.b.SenderRole {
	case protocol.Beacon:
		sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.b.SenderGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.b.SenderGUID, sw.err)
			return
		}
		sw.publicKey = sw.beacon.PublicKey
		sw.aesKey = sw.beacon.SessionKey
		sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	case protocol.Node:
		sw.node, sw.err = sw.ctx.ctx.db.SelectNode(sw.b.SenderGUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select node %X failed %s", sw.b.SenderGUID, sw.err)
			return
		}
		sw.publicKey = sw.node.PublicKey
		sw.aesKey = sw.node.SessionKey
		sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	default:
		panic("invalid b.SenderRole")
	}
	// verify
	sw.buffer.Reset()
	sw.buffer.Write(sw.b.GUID)
	sw.buffer.Write(sw.b.Message)
	sw.buffer.Write(sw.b.Hash)
	sw.buffer.WriteByte(sw.b.SenderRole.Byte())
	sw.buffer.Write(sw.b.SenderGUID)
	if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.b.Signature) {
		sw.ctx.logf(logger.Exploit, "invalid broadcast signature %s guid: %X",
			sw.b.SenderRole, sw.b.SenderGUID)
		return
	}
	// check is handled
	if !sw.ctx.checkBroadcastGUID(sw.b.SenderRole, sw.b.GUID) {
		return
	}
	// decrypt
	sw.b.Message, sw.err = aes.CBCDecrypt(sw.b.Message, sw.aesKey, sw.aesIV)
	if sw.err != nil {
		sw.ctx.logf(logger.Exploit, "decrypt %s guid: %X broadcast failed: %s",
			sw.b.SenderRole, sw.b.SenderGUID, sw.err)
		return
	}
	// check hash
	sw.hash.Reset()
	sw.hash.Write(sw.b.Message)
	if !bytes.Equal(sw.hash.Sum(nil), sw.b.Hash) {
		sw.ctx.logf(logger.Exploit, "%s guid: %X broadcast with wrong hash",
			sw.b.SenderRole, sw.b.SenderGUID)
		return
	}
	sw.ctx.ctx.handleBroadcast(sw.b.Message, sw.b.SenderRole, sw.b.SenderGUID)
}

func (sw *syncerWorker) queryBeaconMessage() (*protocol.SyncReply, error) {
	sw.buffer.Reset()
	sw.err = sw.msgpackEncoder.Encode(sw.syncQuery)
	if sw.err != nil {
		return nil, sw.err
	}
	sw.syncQueryBytes = sw.buffer.Bytes()
	// query
	for i := 0; i < sw.ctx.retryTimes+1; i++ {
		sw.sClients = sw.ctx.Clients()
		if len(sw.sClients) == 0 {
			return nil, protocol.ErrNoSyncerClients
		}
		for _, sw.sClient = range sw.sClients {
			sw.syncReply, sw.err = sw.sClient.QueryBeaconMessage(sw.syncQueryBytes)
			if sw.err != nil {
				sw.ctx.logln(logger.Warning, "query beacon message failed:", sw.err)
				continue
			}
			if sw.syncReply.Err == nil {
				return sw.syncReply, nil
			} else {
				sw.ctx.logln(logger.Warning, "query beacon message with error:", sw.syncReply.Err)
			}
			select {
			case <-sw.ctx.stopSignal:
				return nil, protocol.ErrWorkerStopped
			default:
			}
		}
		select {
		case <-sw.ctx.stopSignal:
			return nil, protocol.ErrWorkerStopped
		default:
		}
		time.Sleep(sw.ctx.retryInterval)
	}
	return nil, protocol.ErrNotExistMessage
}

func (sw *syncerWorker) queryNodeMessage() (*protocol.SyncReply, error) {
	sw.buffer.Reset()
	sw.err = sw.msgpackEncoder.Encode(sw.syncQuery)
	if sw.err != nil {
		return nil, sw.err
	}
	sw.syncQueryBytes = sw.buffer.Bytes()
	for i := 0; i < sw.ctx.retryTimes+1; i++ {
		sw.sClients = sw.ctx.Clients()
		if len(sw.sClients) == 0 {
			return nil, protocol.ErrNoSyncerClients
		}
		for _, sw.sClient = range sw.sClients {
			sw.syncReply, sw.err = sw.sClient.QueryNodeMessage(sw.syncQueryBytes)
			if sw.err != nil {
				sw.ctx.logln(logger.Warning, "query node message failed:", sw.err)
				continue
			}
			if sw.syncReply.Err == nil {
				return sw.syncReply, nil
			} else {
				sw.ctx.logln(logger.Warning, "query node message with error:", sw.syncReply.Err)
			}
			select {
			case <-sw.ctx.stopSignal:
				return nil, protocol.ErrWorkerStopped
			default:
			}
		}
		select {
		case <-sw.ctx.stopSignal:
			return nil, protocol.ErrWorkerStopped
		default:
		}
		time.Sleep(sw.ctx.retryInterval)
	}
	return nil, protocol.ErrNotExistMessage
}

func (sw *syncerWorker) handleSyncTask() {
	if sw.ctx.isBlock() {
		sw.ctx.addSyncTask(sw.st)
		return
	}
	defer sw.ctx.blockDone()
	sw.buffer.Reset()
	_, _ = sw.base64Encoder.Write(sw.st.GUID)
	_ = sw.base64Encoder.Close()
	sw.roleGUID = sw.buffer.String()
	if sw.ctx.isSync(sw.st.Role, sw.roleGUID) {
		return
	}
	defer sw.ctx.syncDone(sw.st.Role, sw.roleGUID)
	// set key
	switch sw.st.Role {
	case protocol.Beacon:
		sw.beacon, sw.err = sw.ctx.ctx.db.SelectBeacon(sw.st.GUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select beacon %X failed %s", sw.st.GUID, sw.err)
			return
		}
		sw.publicKey = sw.beacon.PublicKey
		sw.aesKey = sw.beacon.SessionKey
		sw.aesIV = sw.beacon.SessionKey[:aes.IVSize]
	case protocol.Node:
		sw.node, sw.err = sw.ctx.ctx.db.SelectNode(sw.st.GUID)
		if sw.err != nil {
			sw.ctx.logf(logger.Warning, "select node %X failed %s", sw.st.GUID, sw.err)
			return
		}
		sw.publicKey = sw.node.PublicKey
		sw.aesKey = sw.node.SessionKey
		sw.aesIV = sw.node.SessionKey[:aes.IVSize]
	default: // <safe>
		panic("invalid st.Role")
	}
	// sync message loop
	for {
		switch sw.st.Role {
		case protocol.Beacon:
			sw.beaconSyncer, sw.err = sw.ctx.ctx.db.SelectBeaconSyncer(sw.st.GUID)
			if sw.err != nil {
				sw.ctx.logf(logger.Warning, "select beacon syncer %X failed %s", sw.st.GUID, sw.err)
				return
			}
			sw.roleSend = sw.beaconSyncer.BeaconSend
			sw.ctrlReceive = sw.beaconSyncer.CtrlRecv
		case protocol.Node:
			sw.nodeSyncer, sw.err = sw.ctx.ctx.db.SelectNodeSyncer(sw.st.GUID)
			if sw.err != nil {
				sw.ctx.logf(logger.Warning, "select node syncer %X failed %s", sw.st.GUID, sw.err)
				return
			}
			sw.roleSend = sw.nodeSyncer.NodeSend
			sw.ctrlReceive = sw.nodeSyncer.CtrlRecv
		default: // <safe>
			panic("invalid st.Role")
		}
		// don't need sync
		if sw.roleSend <= sw.ctrlReceive {
			return
		}
		// query message
		sw.syncQuery.GUID = sw.st.GUID
		sw.syncQuery.Index = sw.ctrlReceive
		switch sw.st.Role {
		case protocol.Beacon:
			sw.syncReply, sw.err = sw.queryBeaconMessage()
		case protocol.Node:
			sw.syncReply, sw.err = sw.queryNodeMessage()
		default: // <safe>
			panic("invalid st.Role")
		}
		switch sw.err {
		case nil:
			// verify
			sw.buffer.Reset()
			sw.buffer.Write(sw.syncReply.GUID)
			sw.buffer.Write(convert.Uint64ToBytes(sw.ctrlReceive))
			sw.buffer.Write(sw.syncReply.Message)
			sw.buffer.Write(sw.syncReply.Hash)
			sw.buffer.WriteByte(sw.st.Role.Byte())
			sw.buffer.Write(sw.st.GUID)
			sw.buffer.WriteByte(protocol.Ctrl.Byte())
			sw.buffer.Write(protocol.CtrlGUID)
			if !ed25519.Verify(sw.publicKey, sw.buffer.Bytes(), sw.syncReply.Signature) {
				sw.ctx.logf(logger.Exploit, "invalid sync reply signature %s guid: %X",
					sw.st.Role, sw.st.GUID)
				continue
			}
			// decrypt
			sw.syncReply.Message, sw.err = aes.CBCDecrypt(sw.syncReply.Message, sw.aesKey, sw.aesIV)
			if sw.err != nil {
				sw.ctx.logf(logger.Exploit, "decrypt %s guid: %X sync reply failed: %s",
					sw.st.Role, sw.st.GUID, sw.err)
				continue
			}
			// check hash
			sw.hash.Reset()
			sw.hash.Write(sw.syncReply.Message)
			if !bytes.Equal(sw.hash.Sum(nil), sw.syncReply.Hash) {
				sw.ctx.logf(logger.Exploit, "%s guid: %X sync reply with wrong hash",
					sw.st.Role, sw.st.GUID)
				continue
			}
			sw.ctx.ctx.handleMessage(sw.syncReply.Message, sw.st.Role, sw.st.GUID, sw.ctrlReceive)
		case protocol.ErrNoSyncerClients:
			sw.ctx.log(logger.Warning, sw.err)
			return
		case protocol.ErrNotExistMessage:
			sw.ctx.logf(logger.Error, "%s guid: %X index: %d %s",
				sw.st.Role, sw.st.GUID, sw.ctrlReceive, sw.err)
		case protocol.ErrWorkerStopped:
			return
		default:
			panic("syncer.worker(): handle invalid query() error")
		}
		// update height and notice
		switch sw.st.Role {
		case protocol.Beacon:
			sw.err = sw.ctx.ctx.db.UpdateBSCtrlReceive(sw.st.GUID, sw.ctrlReceive+1)
		case protocol.Node:
			sw.err = sw.ctx.ctx.db.UpdateNSCtrlReceive(sw.st.GUID, sw.ctrlReceive+1)
		default:
			panic("invalid st.Role")
		}
		if sw.err != nil {
			sw.ctx.logf(logger.Error, "%s guid: %X %s", sw.st.Role, sw.st.GUID, sw.err)
			return
		}
		// notice node to delete message
		sw.ctx.ctx.sender.SyncReceive(sw.st.Role, sw.st.GUID, sw.ctrlReceive)
		select {
		case <-sw.ctx.stopSignal:
			return
		default:
		}
	}
}

func (sw *syncerWorker) Work() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer.worker() panic:", r)
			sw.ctx.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			sw.ctx.wg.Add(1)
			go sw.Work()
		}
		sw.ctx.wg.Done()
	}()
	// init buffer
	// protocol.SyncReceive buffer cap = guid.Size + 8 + 1 + guid.Size
	minBufferSize := 2*guid.Size + 9
	sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
	sw.msgpackEncoder = msgpack.NewEncoder(sw.buffer)
	sw.base64Encoder = base64.NewEncoder(base64.StdEncoding, sw.buffer)
	sw.hash = sha256.New()
	sw.syncQuery = &protocol.SyncQuery{}
	sw.syncReply = &protocol.SyncReply{}
	// start handle task
	for {
		// check buffer capacity
		if sw.buffer.Cap() > sw.maxBufferSize {
			sw.buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		case sw.sr = <-sw.ctx.syncReceiveQueue:
			sw.handleSyncReceive()
		case sw.ss = <-sw.ctx.syncSendQueue:
			sw.handleSyncSend()
		case sw.b = <-sw.ctx.broadcastQueue:
			sw.handleBroadcast()
		case sw.st = <-sw.ctx.syncTaskQueue:
			sw.handleSyncTask()
		case <-sw.ctx.stopSignal:
			return
		}
	}
}

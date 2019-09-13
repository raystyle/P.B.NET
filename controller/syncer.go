package controller

import (
	"bytes"
	"encoding/base64"
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

const (
	syncerNode   = 0
	syncerBeacon = 1
)

type syncer struct {
	ctx              *CTRL
	maxBufferSize    int
	workerQueueSize  int
	maxBlockWorker   int
	retryTimes       int
	retryInterval    time.Duration
	broadcastTimeout float64
	maxSyncer        int
	maxSyncerM       sync.Mutex
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
	syncStatus   [2]map[string]bool
	syncStatusM  [2]sync.Mutex
	blockWorker  int
	blockWorkerM sync.Mutex

	clients    map[string]*sClient // key=base64(guid)
	clientsRWM sync.RWMutex

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
		maxBufferSize:    cfg.MaxBufferSize,
		maxSyncer:        cfg.MaxSyncerClient,
		workerQueueSize:  cfg.SyncerQueueSize,
		maxBlockWorker:   cfg.SyncerWorker - cfg.ReserveWorker,
		retryTimes:       cfg.RetryTimes,
		retryInterval:    cfg.RetryInterval,
		broadcastTimeout: cfg.BroadcastTimeout.Seconds(),
		broadcastQueue:   make(chan *protocol.Broadcast, cfg.SyncerQueueSize),
		syncSendQueue:    make(chan *protocol.SyncSend, cfg.SyncerQueueSize),
		syncReceiveQueue: make(chan *protocol.SyncReceive, cfg.SyncerQueueSize),
		syncTaskQueue:    make(chan *protocol.SyncTask, cfg.SyncerQueueSize),
		clients:          make(map[string]*sClient),
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
		syncer.wg.Add(1)
		go syncer.worker()
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
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	sClientsLen := len(syncer.clients)
	if sClientsLen >= syncer.getMaxSyncer() {
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
	syncer.clients[key] = sClient
	return nil
}

func (syncer *syncer) sClients() map[string]*sClient {
	syncer.clientsRWM.RLock()
	l := len(syncer.clients)
	if l == 0 {
		syncer.clientsRWM.RUnlock()
		return nil
	}
	// copy map
	sClients := make(map[string]*sClient, l)
	for key, client := range syncer.clients {
		sClients[key] = client
	}
	syncer.clientsRWM.RUnlock()
	return sClients
}

// getMaxSyncer is used to get current max syncer number
func (syncer *syncer) getMaxSyncer() int {
	syncer.maxSyncerM.Lock()
	maxSyncer := syncer.maxSyncer
	syncer.maxSyncerM.Unlock()
	return maxSyncer
}

// watcher is used to check connect nodes number
// connected nodes number < syncer.maxSyncer, try to connect more node
func (syncer *syncer) watcher() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer watcher panic:", r)
			syncer.log(logger.FATAL, err)
			// restart watcher
			syncer.wg.Add(1)
			go syncer.watcher()
		}
		syncer.wg.Done()
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	isMax := func() bool {
		// get current syncer client number
		syncer.clientsRWM.RLock()
		sClientsLen := len(syncer.clients)
		syncer.clientsRWM.RUnlock()
		return sClientsLen >= syncer.getMaxSyncer()
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

func (syncer *syncer) logf(l logger.Level, format string, log ...interface{}) {
	syncer.ctx.Printf(l, "syncer", format, log...)
}

func (syncer *syncer) log(l logger.Level, log ...interface{}) {
	syncer.ctx.Print(l, "syncer", log...)
}

func (syncer *syncer) logln(l logger.Level, log ...interface{}) {
	syncer.ctx.Println(l, "syncer", log...)
}

// task from syncer client

func (syncer *syncer) addBroadcast(br *protocol.Broadcast) {
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

func (syncer *syncer) addSyncSend(ss *protocol.SyncSend) {
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

func (syncer *syncer) addSyncReceive(sr *protocol.SyncReceive) {
	if len(syncer.broadcastQueue) == syncer.workerQueueSize {
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

// check xxx Token is used to check xxx is been handled
// xxx = broadcast, sync send, sync receive
// just tell others, but they can still send it by force

func (syncer *syncer) checkBroadcastToken(role byte, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch protocol.Role(role) {
	case protocol.Beacon:
		i = syncerBeacon
	case protocol.Node:
		i = syncerNode
	default:
		// tODO log not panic
		panic("invalid role")
	}
	syncer.broadcastGUIDRWM[i].RLock()
	_, ok := syncer.broadcastGUID[i][key]
	syncer.broadcastGUIDRWM[i].RUnlock()
	return !ok
}

func (syncer *syncer) checkSyncSendToken(role byte, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch protocol.Role(role) {
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

func (syncer *syncer) checkSyncReceiveToken(role byte, guid []byte) bool {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.broadcastTimeout {
		return false
	}
	key := base64.StdEncoding.EncodeToString(guid)
	i := 0
	switch protocol.Role(role) {
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
			syncer.log(logger.FATAL, err)
			// restart guid cleaner
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

func (syncer *syncer) worker() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer.worker() panic:", r)
			syncer.log(logger.FATAL, err)
			// restart worker
			syncer.wg.Add(1)
			go syncer.worker()
		}
		syncer.wg.Done()
	}()
	var (
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

		// temp
		nodeSyncer     *mNodeSyncer
		beaconSyncer   *mBeaconSyncer
		roleGUID       string
		roleSend       uint64
		ctrlReceive    uint64
		sClients       map[string]*sClient
		sClient        *sClient
		syncQueryBytes []byte
		err            error
	)
	// init buffer
	// protocol.SyncReceive buffer cap = guid.SIZE + 8 + 1 + guid.SIZE
	minBufferSize := 2*guid.SIZE + 9
	buffer := bytes.NewBuffer(make([]byte, minBufferSize))
	encoder := msgpack.NewEncoder(buffer)
	syncQuery := &protocol.SyncQuery{}
	syncReply := &protocol.SyncReply{}
	// query is used to query message by index
	query := func() (*protocol.SyncReply, error) {
		buffer.Reset()
		err = encoder.Encode(syncQuery)
		if err != nil {
			return nil, err
		}
		// breadth-first search
		for i := 0; i < syncer.retryTimes+1; i++ {
			sClients = syncer.sClients()
			if len(sClients) == 0 {
				return nil, protocol.ErrNoSyncerClients
			}
			syncQueryBytes = buffer.Bytes()
			for _, sClient = range sClients {
				syncReply, err = sClient.QueryMessage(syncQueryBytes)
				if err != nil {
					// TODO print error
					continue
				}
				if syncReply.Err == nil {
					return syncReply, nil
				} else {
					// TODO print error
				}
				select {
				case <-syncer.stopSignal:
					return nil, protocol.ErrWorkerStopped
				default:
				}
			}
			select {
			case <-syncer.stopSignal:
				return nil, protocol.ErrWorkerStopped
			default:
			}
			time.Sleep(syncer.retryInterval)
		}
		return nil, protocol.ErrNoMessage
	}
	// start handle
	for {
		// check buffer capacity
		if buffer.Cap() > syncer.maxBufferSize {
			buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		// ----------------------handle sync receive-----------------------
		case sr = <-syncer.syncReceiveQueue:
			// check role and set key
			switch sr.ReceiverRole {
			case protocol.Beacon:
				beacon, err = syncer.ctx.db.SelectBeacon(sr.ReceiverGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select beacon %X failed %s",
						sr.ReceiverGUID, err)
					continue
				}
				publicKey = beacon.PublicKey
			case protocol.Node:
				node, err = syncer.ctx.db.SelectNode(sr.ReceiverGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select node %X failed %s",
						sr.ReceiverGUID, err)
					continue
				}
				publicKey = node.PublicKey
			default:
				panic("invalid sr.ReceiverRole")
			}
			// must first verify
			buffer.Reset()
			buffer.Write(sr.GUID)
			buffer.Write(convert.Uint64ToBytes(sr.Height))
			buffer.WriteByte(sr.ReceiverRole.Byte())
			buffer.Write(sr.ReceiverGUID)
			if !ed25519.Verify(publicKey, buffer.Bytes(), sr.Signature) {
				syncer.logf(logger.EXPLOIT, "invalid sync receive signature %s guid: %X",
					sr.ReceiverRole, sr.ReceiverGUID)
				continue
			}
			if !syncer.checkSyncReceiveGUID(sr.ReceiverRole, sr.GUID) {
				continue
			}
			sr.Height += 1
			// update role receive
			switch sr.ReceiverRole {
			case protocol.Beacon:
				err = syncer.ctx.db.UpdateBSBeaconReceive(sr.ReceiverGUID, sr.Height)
				if err != nil {
					syncer.logf(logger.WARNING, "update %X beacon receive failed %s",
						sr.ReceiverGUID, err)
				}
			case protocol.Node:
				err = syncer.ctx.db.UpdateNSNodeReceive(sr.ReceiverGUID, sr.Height)
				if err != nil {
					syncer.logf(logger.WARNING, "update %X node receive failed %s",
						sr.ReceiverGUID, err)
				}
			}
		// -----------------------handle sync send-------------------------
		case ss = <-syncer.syncSendQueue:
			// set key
			switch ss.SenderRole {
			case protocol.Beacon:
				beacon, err = syncer.ctx.db.SelectBeacon(ss.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select beacon %X failed %s",
						ss.SenderGUID, err)
					continue
				}
				publicKey = beacon.PublicKey
				aesKey = beacon.SessionKey[:aes.Bit256]
				aesIV = beacon.SessionKey[aes.Bit256:]
			case protocol.Node:
				node, err = syncer.ctx.db.SelectNode(ss.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select node %X failed %s",
						ss.SenderGUID, err)
					continue
				}
				publicKey = node.PublicKey
				aesKey = node.SessionKey[:aes.Bit256]
				aesIV = node.SessionKey[aes.Bit256:]
			default:
				panic("invalid ss.SenderRole")
			}
			// verify
			buffer.Reset()
			buffer.Write(ss.GUID)
			buffer.Write(convert.Uint64ToBytes(ss.Height))
			buffer.Write(ss.Message)
			buffer.WriteByte(ss.SenderRole.Byte())
			buffer.Write(ss.SenderGUID)
			buffer.WriteByte(ss.ReceiverRole.Byte())
			buffer.Write(ss.ReceiverGUID)
			if !ed25519.Verify(publicKey, buffer.Bytes(), ss.Signature) {
				syncer.logf(logger.EXPLOIT, "invalid sync send signature %s guid: %X",
					ss.SenderRole, ss.SenderGUID)
				continue
			}
			if !syncer.checkSyncSendGUID(ss.SenderRole, ss.GUID) {
				continue
			}
			ss.Height += 1 // index -> height
			// update role send
			switch ss.SenderRole {
			case protocol.Beacon:
				err = syncer.ctx.db.UpdateBSBeaconSend(ss.SenderGUID, ss.Height)
				syncer.logf(logger.WARNING, "update %X beacon send failed %s",
					ss.SenderGUID, err)
			case protocol.Node:
				err = syncer.ctx.db.UpdateNSNodeSend(ss.SenderGUID, ss.Height)
				syncer.logf(logger.WARNING, "update %X node send failed %s",
					ss.SenderGUID, err)
			}
			// lock role
			roleGUID = base64.StdEncoding.EncodeToString(ss.SenderGUID)
			if syncer.isSync(ss.SenderRole, roleGUID) {
				continue
			}
			// select role send & controller receive
			// must select again, because maybe update
			// role send at the same time
			switch ss.SenderRole {
			case protocol.Beacon:
				beaconSyncer, err = syncer.ctx.db.SelectBeaconSyncer(ss.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select beacon syncer %X failed %s",
						ss.SenderGUID, err)
					syncer.syncDone(ss.SenderRole, roleGUID)
					continue
				}
				roleSend = beaconSyncer.BeaconSend
				ctrlReceive = beaconSyncer.CtrlRecv
			case protocol.Node:
				nodeSyncer, err = syncer.ctx.db.SelectNodeSyncer(ss.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select node syncer %X failed %s",
						ss.SenderGUID, err)
					syncer.syncDone(ss.SenderRole, roleGUID)
					continue
				}
				roleSend = nodeSyncer.NodeSend
				ctrlReceive = nodeSyncer.CtrlRecv
			}
			// check height
			sub := roleSend - ctrlReceive
			switch {
			case sub < 1: // received message
				syncer.syncDone(ss.SenderRole, roleGUID)
			case sub == 1: // only one message, handle it
				ss.Message, err = aes.CBCDecrypt(ss.Message, aesKey, aesIV)
				if err != nil {
					syncer.logf(logger.EXPLOIT, "decrypt %s guid: %X sync send failed: %s",
						ss.SenderRole, ss.SenderGUID, err)
					syncer.syncDone(ss.SenderRole, roleGUID)
					continue
				}
				syncer.ctx.handleMessage(ss.Message, ss.SenderRole, ss.SenderGUID, roleSend-1)
				// update controller receive
				switch ss.SenderRole {
				case protocol.Beacon:
					err = syncer.ctx.db.UpdateBSCtrlReceive(ss.SenderGUID, roleSend)
					if err != nil {
						syncer.logf(logger.WARNING, "update beacon syncer %X ctrl send failed %s",
							ss.SenderGUID, err)
					}
				case protocol.Node:
					err = syncer.ctx.db.UpdateNSCtrlReceive(ss.SenderGUID, roleSend)
					if err != nil {
						syncer.logf(logger.WARNING, "update node syncer %X ctrl send failed %s",
							ss.SenderGUID, err)
					}
				}
				syncer.syncDone(ss.SenderRole, roleGUID)
				// notice node to delete message
				syncer.ctx.sender.syncReceive(ss.SenderRole, ss.SenderGUID, roleSend-1)
			case sub > 1: // get old message and need sync more message
				syncer.addSyncTask(&protocol.SyncTask{
					Role: ss.SenderRole,
					GUID: ss.SenderGUID,
				})
				syncer.syncDone(ss.SenderRole, roleGUID)
			}
		// -----------------------handle broadcast-------------------------
		case b = <-syncer.broadcastQueue:
			// set key
			switch b.SenderRole {
			case protocol.Beacon:
				beacon, err = syncer.ctx.db.SelectBeacon(b.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select beacon %X failed %s",
						b.SenderGUID, err)
					continue
				}
				publicKey = beacon.PublicKey
				aesKey = beacon.SessionKey[:aes.Bit256]
				aesIV = beacon.SessionKey[aes.Bit256:]
			case protocol.Node:
				node, err = syncer.ctx.db.SelectNode(b.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select node %X failed %s",
						b.SenderGUID, err)
					continue
				}
				publicKey = node.PublicKey
				aesKey = node.SessionKey[:aes.Bit256]
				aesIV = node.SessionKey[aes.Bit256:]
			default:
				panic("invalid b.SenderRole")
			}
			// verify
			buffer.Reset()
			buffer.Write(b.GUID)
			buffer.Write(b.Message)
			buffer.WriteByte(b.SenderRole.Byte())
			buffer.Write(b.SenderGUID)
			buffer.WriteByte(b.ReceiverRole.Byte())
			if !ed25519.Verify(publicKey, buffer.Bytes(), b.Signature) {
				syncer.logf(logger.EXPLOIT, "invalid broadcast signature %s guid: %X",
					b.SenderRole, b.SenderGUID)
				continue
			}
			if !syncer.checkBroadcastGUID(b.SenderRole, b.GUID) {
				continue
			}
			b.Message, err = aes.CBCDecrypt(b.Message, aesKey, aesIV)
			if err != nil {
				syncer.logf(logger.EXPLOIT, "decrypt %s guid: %X broadcast failed: %s",
					b.SenderRole, b.SenderGUID, err)
				continue
			}
			syncer.ctx.handleBroadcast(b.Message, b.SenderRole, b.SenderGUID)
			// -----------------------handle sync task-------------------------
		case st = <-syncer.syncTaskQueue:
			if syncer.isBlock() {
				syncer.addSyncTask(st)
				continue
			}
			roleGUID = base64.StdEncoding.EncodeToString(st.GUID)
			if syncer.isSync(st.Role, roleGUID) {
				continue
			}
			// set key
			switch st.Role {
			case protocol.Beacon:
				beacon, err = syncer.ctx.db.SelectBeacon(st.GUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select beacon %X failed %s",
						st.GUID, err)
					syncer.syncDone(st.Role, roleGUID)
					syncer.blockDone()
					continue
				}
				publicKey = beacon.PublicKey
				aesKey = beacon.SessionKey[:aes.Bit256]
				aesIV = beacon.SessionKey[aes.Bit256:]
			case protocol.Node:
				node, err = syncer.ctx.db.SelectNode(st.GUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select node %X failed %s",
						st.GUID, err)
					syncer.syncDone(st.Role, roleGUID)
					syncer.blockDone()
					continue
				}
				publicKey = node.PublicKey
				aesKey = node.SessionKey[:aes.Bit256]
				aesIV = node.SessionKey[aes.Bit256:]
			default: // <safe>
				syncer.syncDone(st.Role, roleGUID)
				syncer.blockDone()
				panic("invalid st.SenderRole")
			}
			// sync message loop
		syncLoop:
			for {
				switch st.Role {
				case protocol.Beacon:
					beaconSyncer, err = syncer.ctx.db.SelectBeaconSyncer(st.GUID)
					if err != nil {
						syncer.logf(logger.WARNING, "select beacon syncer %X failed %s",
							st.GUID, err)
						break syncLoop
					}
					roleSend = beaconSyncer.BeaconSend
					ctrlReceive = beaconSyncer.CtrlRecv
				case protocol.Node:
					nodeSyncer, err = syncer.ctx.db.SelectNodeSyncer(st.GUID)
					if err != nil {
						syncer.logf(logger.WARNING, "select node syncer %X failed %s",
							st.GUID, err)
						break syncLoop
					}
					roleSend = nodeSyncer.NodeSend
					ctrlReceive = nodeSyncer.CtrlRecv
				}
				// don't need sync
				if roleSend <= ctrlReceive {
					break
				}
				syncQuery.Role = st.Role
				syncQuery.GUID = st.GUID
				syncQuery.Height = ctrlReceive
				syncReply, err = query()
				switch err {
				case nil:
					// verify  // see protocol.SyncSend
					buffer.Reset()
					buffer.Write(syncReply.GUID)
					buffer.Write(convert.Uint64ToBytes(ctrlReceive))
					buffer.Write(syncReply.Message)
					buffer.WriteByte(st.Role.Byte())
					buffer.Write(st.GUID)
					buffer.WriteByte(protocol.Ctrl.Byte())
					buffer.Write(protocol.CtrlGUID)
					if !ed25519.Verify(publicKey, buffer.Bytes(), syncReply.Signature) {
						syncer.logf(logger.EXPLOIT, "invalid sync reply signature %s guid: %X",
							st.Role, st.GUID)
					}
					syncReply.Message, err = aes.CBCDecrypt(syncReply.Message, aesKey, aesIV)
					if err != nil {
						syncer.logf(logger.EXPLOIT, "decrypt %s guid: %X sync reply failed: %s",
							st.Role, st.GUID, err)
						continue
					}
					syncer.ctx.handleMessage(syncReply.Message, st.Role, st.GUID, ctrlReceive)
				case protocol.ErrNoSyncerClients:
					syncer.log(logger.WARNING, err)
					break syncLoop
				case protocol.ErrNoMessage:
					syncer.logf(logger.ERROR, "%s guid: %X index: %d %s",
						st.Role, st.GUID, ctrlReceive, err)
				case protocol.ErrWorkerStopped:
					return
				default:
					syncer.syncDone(st.Role, roleGUID)
					syncer.blockDone()
					panic("syncer.worker(): handle invalid query() error")
				}
				// update height and notice
				switch st.Role {
				case protocol.Beacon:
					err = syncer.ctx.db.UpdateBSCtrlReceive(st.GUID, ctrlReceive+1)
					if err != nil {
						syncer.logf(logger.ERROR, "%s guid: %X %s", st.Role, st.GUID, err)
						break syncLoop
					}
				case protocol.Node:
					err = syncer.ctx.db.UpdateNSCtrlReceive(st.GUID, ctrlReceive+1)
					if err != nil {
						syncer.logf(logger.ERROR, "%s guid: %X %s", st.Role, st.GUID, err)
						break syncLoop
					}
				}
				// notice node to delete message
				syncer.ctx.sender.syncReceive(st.Role, st.GUID, ctrlReceive)
				select {
				case <-syncer.stopSignal:
					return
				default:
				}
			}
			syncer.syncDone(st.Role, roleGUID)
			syncer.blockDone()
		// reply, err := query(sync_query)
		case <-syncer.stopSignal:
			return
		}
	}
}

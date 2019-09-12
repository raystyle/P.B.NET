package controller

import (
	"bytes"
	"encoding/base64"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/guid"
	"project/internal/protocol"
)

const (
	syncerNode   = 0
	syncerBeacon = 1
)

type syncer struct {
	ctx              *CTRL
	maxBufferSize    int
	maxSyncer        int
	workerQueueSize  int
	retryTimes       int
	retryInterval    time.Duration
	broadcastTimeout float64
	configsM         sync.Mutex
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
	// runtime
	sClients    map[string]*sClient
	sClientsRWM sync.RWMutex
	stopSignal  chan struct{}
	wg          sync.WaitGroup
}

func newSyncer(ctx *CTRL, cfg *Config) (*syncer, error) {
	// check config
	if cfg.MaxBufferSize < 4096 {
		return nil, errors.New("max buffer size < 4096")
	}
	if cfg.MaxSyncer < 1 {
		return nil, errors.New("max syncer < 1")
	}
	if cfg.WorkerNumber < 2 {
		return nil, errors.New("worker number < 2")
	}
	if cfg.WorkerQueueSize < 512 {
		return nil, errors.New("worker task queue size < 512")
	}
	if cfg.ReserveWorker >= cfg.WorkerNumber {
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
		maxSyncer:        cfg.MaxSyncer,
		workerQueueSize:  cfg.WorkerQueueSize,
		retryTimes:       cfg.RetryTimes,
		retryInterval:    cfg.RetryInterval,
		broadcastTimeout: cfg.BroadcastTimeout.Seconds(),
		broadcastQueue:   make(chan *protocol.Broadcast, cfg.WorkerQueueSize),
		syncSendQueue:    make(chan *protocol.SyncSend, cfg.WorkerQueueSize),
		syncReceiveQueue: make(chan *protocol.SyncReceive, cfg.WorkerQueueSize),
		syncTaskQueue:    make(chan *protocol.SyncTask, cfg.WorkerQueueSize),
		sClients:         make(map[string]*sClient),
		stopSignal:       make(chan struct{}),
	}
	for i := 0; i < 2; i++ {
		syncer.broadcastGUID[i] = make(map[string]int64)
		syncer.syncSendGUID[i] = make(map[string]int64)
		syncer.syncReceiveGUID[i] = make(map[string]int64)
		syncer.syncStatus[i] = make(map[string]bool)
	}
	for i := 0; i < cfg.WorkerNumber; i++ {
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

func (syncer *syncer) Connect(cfg *clientCfg) {

}

// watcher is used to check connect nodes number
// connected nodes number < syncer.maxSyncer, try to connect more node
func (syncer *syncer) watcher() {
	defer syncer.wg.Done()

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
	defer syncer.wg.Done()
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

// only one role is synchronized at the same time
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

// set sync flag
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

func (syncer *syncer) blockWorkerDone() {
	syncer.blockWorkerM.Lock()
	syncer.blockWorker -= 1
	syncer.blockWorkerM.Unlock()
}

func (syncer *syncer) worker() {
	defer syncer.wg.Done()
	var (
		// task
		b  *protocol.Broadcast
		ss *protocol.SyncSend
		sr *protocol.SyncReceive
		st *protocol.SyncTask
		// key
		// nodeKey   *mNode
		// beaconKey *mBeacon
		// temp
		// roleGUID    string
		// roleSend    uint64
		// roleReceive uint64
		// err         error
	)
	// init buffer
	// protocol.SyncReceive buffer cap = guid.SIZE + 8 + 1 + guid.SIZE
	minBufferSize := 2*guid.SIZE + 9
	buffer := bytes.NewBuffer(make([]byte, minBufferSize))
	/*
		encoder := msgpack.NewEncoder(buffer)

			query := func(r *protocol.SyncQuery) (*protocol.SyncReply, error) {
				buffer.Reset()
				err = encoder.Encode(r)
				if err != nil {
					return nil, err
				}
				//

				return nil, protocol.ErrNoMessage
			}
	*/
	// start handle
	for {
		// check buffer capacity
		if buffer.Cap() > syncer.maxBufferSize {
			buffer = bytes.NewBuffer(make([]byte, minBufferSize))
		}
		select {
		// ----------------------handle sync receive-----------------------
		case sr = <-syncer.syncReceiveQueue:

		// -----------------------handle sync send-------------------------
		case ss = <-syncer.syncSendQueue:

		// -----------------------handle broadcast-------------------------
		case b = <-syncer.broadcastQueue:

		// -----------------------handle sync task-------------------------
		case st = <-syncer.syncTaskQueue:
			// reply, err := query(sync_query)
		case <-syncer.stopSignal:
			return
		}
	}
}

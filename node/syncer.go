package node

import (
	"encoding/base64"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type syncer struct {
	ctx              *Node
	maxBufferSize    int
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
	broadcastGUID    [3]map[string]int64
	broadcastGUIDRWM [3]sync.RWMutex
	// -----------------handle sync message-----------------------
	syncSendQueue      chan *protocol.Send
	syncSendGUID       [3]map[string]int64
	syncSendGUIDRWM    [3]sync.RWMutex
	syncReceiveQueue   chan *protocol.SyncReceive
	syncReceiveGUID    [3]map[string]int64
	syncReceiveGUIDRWM [3]sync.RWMutex
	// -------------------handle sync task------------------------
	syncTaskQueue chan *protocol.SyncTask

	// connected node key=base64(guid)
	sClients    map[string]*sClient
	sClientsRWM sync.RWMutex

	// incoming role connections
	ctrlConn       *ctrlConn // only one if has two Exploit!!!!
	ctrlConnM      sync.Mutex
	nodeConns      map[string]*nodeConn
	nodeConnsRWM   sync.RWMutex
	beaconConns    map[string]*beaconConn
	beaconConnsRWM sync.RWMutex

	inClose    int32
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSyncer(ctx *Node, cfg *Config) (*syncer, error) {
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
		maxSyncerClient:  cfg.MaxSyncerClient,
		workerQueueSize:  cfg.SyncerQueueSize,
		maxBlockWorker:   cfg.SyncerWorker - cfg.ReserveWorker,
		retryTimes:       cfg.RetryTimes,
		retryInterval:    cfg.RetryInterval,
		broadcastTimeout: cfg.BroadcastTimeout.Seconds(),
		broadcastQueue:   make(chan *protocol.Broadcast, cfg.SyncerQueueSize),
		syncSendQueue:    make(chan *protocol.Send, cfg.SyncerQueueSize),
		syncReceiveQueue: make(chan *protocol.SyncReceive, cfg.SyncerQueueSize),
		syncTaskQueue:    make(chan *protocol.SyncTask, cfg.SyncerQueueSize),
		sClients:         make(map[string]*sClient),
		nodeConns:        make(map[string]*nodeConn),
		beaconConns:      make(map[string]*beaconConn),

		stopSignal: make(chan struct{}),
	}

	for i := 0; i < 3; i++ {
		syncer.broadcastGUID[i] = make(map[string]int64)
		syncer.syncSendGUID[i] = make(map[string]int64)
		syncer.syncReceiveGUID[i] = make(map[string]int64)
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

// SyncerClients return connected Nodes
func (syncer *syncer) SyncerClients() map[string]*sClient {
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

func (syncer *syncer) SetCtrlConn(ctrl *ctrlConn) bool {
	syncer.ctrlConnM.Lock()
	defer syncer.ctrlConnM.Unlock()
	if syncer.ctrlConn == nil {
		syncer.ctrlConn = ctrl
		return true
	} else {
		return false
	}
}

func (syncer *syncer) CtrlConn() *ctrlConn {
	syncer.ctrlConnM.Lock()
	cc := syncer.ctrlConn
	syncer.ctrlConnM.Unlock()
	return cc
}

func (syncer *syncer) isClosed() bool {
	return atomic.LoadInt32(&syncer.inClose) != 0
}

func (syncer *syncer) Close() {
	atomic.StoreInt32(&syncer.inClose, 1)
	// disconnect all syncer clients

	// wait close

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
			err := xpanic.Error(r, "syncer watcher panic:")
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

// task from syncer client
func (syncer *syncer) addSyncSend(ss *protocol.Send) {
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
func (syncer *syncer) addSyncReceive(sr *protocol.SyncReceive) {
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
	syncerCtrl   = 0
	syncerNode   = 1
	syncerBeacon = 2
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
	case protocol.Ctrl:
		i = syncerCtrl
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
	case protocol.Ctrl:
		i = syncerCtrl
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
	case protocol.Ctrl:
		i = syncerCtrl
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
	case protocol.Ctrl:
		i = syncerCtrl
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
	case protocol.Ctrl:
		i = syncerCtrl
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
	case protocol.Ctrl:
		i = syncerCtrl
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
			err := xpanic.Error(r, "syncer guid cleaner panic:")
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
			for i := 0; i < 3; i++ {
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

func (syncer *syncer) worker() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error(r, "syncer.worker() panic:")
			syncer.log(logger.Fatal, err)
			// restart worker
			time.Sleep(time.Second)
			syncer.wg.Add(1)
			go syncer.worker()
		}
		syncer.wg.Done()
	}()
}

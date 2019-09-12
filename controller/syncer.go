package controller

import (
	"bytes"
	"encoding/base64"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

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
	maxSyncer        int
	workerQueueSize  int
	retryTimes       int
	retryInterval    time.Duration
	broadcastTimeout float64
	configsRWM       sync.RWMutex
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
	// TODO when remove role need delete it
	syncStatus   [2]map[string]bool
	syncStatusM  [2]sync.Mutex
	blockWorker  int
	blockWorkerM sync.Mutex
	// runtime
	clients    map[string]*sClient // key=base64(guid)
	ClientsRWM sync.RWMutex
	stopSignal chan struct{}
	wg         sync.WaitGroup
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
		clients:          make(map[string]*sClient),
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

// Connect is used to connect node for sync message
func (syncer *syncer) Connect(node *bootstrap.Node, guid []byte) error {
	syncer.ClientsRWM.Lock()
	defer syncer.ClientsRWM.Unlock()
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

// getMaxSyncer is used to get current max syncer number
func (syncer *syncer) getMaxSyncer() int {
	syncer.configsRWM.RLock()
	maxSyncer := syncer.maxSyncer
	syncer.configsRWM.RUnlock()
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
		syncer.ClientsRWM.RLock()
		sClientsLen := len(syncer.clients)
		syncer.ClientsRWM.RUnlock()
		return sClientsLen >= syncer.getMaxSyncer()
	}
	watch := func() {
		if isMax() {
			return
		}
		// select nodes

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

// set sync done
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
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("syncer worker panic:", r)
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
		// st *protocol.SyncTask

		// key
		node      *mNode
		beacon    *mBeacon
		publicKey ed25519.PublicKey
		aesKey    []byte
		aesIV     []byte

		// temp
		ns          *mNodeSyncer
		bs          *mBeaconSyncer
		roleGUID    string
		roleSend    uint64
		ctrlReceive uint64
		err         error
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
				syncer.logf(logger.EXPLOIT, "invalid sync receive role: %s guid: %X",
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
				syncer.logf(logger.EXPLOIT, "invalid sync send role: %s guid: %X",
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
				bs, err = syncer.ctx.db.SelectBeaconSyncer(ss.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select beacon syncer %X failed %s",
						ss.SenderGUID, err)
					continue
				}
				roleSend = bs.BeaconSend
				ctrlReceive = bs.CtrlRecv
			case protocol.Node:
				ns, err = syncer.ctx.db.SelectNodeSyncer(ss.SenderGUID)
				if err != nil {
					syncer.logf(logger.WARNING, "select node syncer %X failed %s",
						ss.SenderGUID, err)
					continue
				}
				roleSend = ns.NodeSend
				ctrlReceive = ns.CtrlRecv
			}
			// check height
			sub := roleSend - ctrlReceive
			switch {
			case sub < 1: // received message
				syncer.syncDone(ss.SenderRole, roleGUID)
			case sub == 1: // only one message, handle it
				ss.Message, err = aes.CBCDecrypt(ss.Message, aesKey, aesIV)
				if err != nil {
					syncer.syncDone(ss.SenderRole, roleGUID)
					syncer.logf(logger.EXPLOIT, "decrypt %s guid: %X message failed: %s",
						ss.SenderRole, ss.SenderGUID, err)
					continue
				}
				syncer.ctx.handleMessage(ss.Message, ss.SenderRole, ss.SenderGUID, roleSend-1)
				// update controller receive
				switch ss.SenderRole {
				case protocol.Beacon:
					err = syncer.ctx.db.UpdateBSCtrlReceive(ss.SenderGUID, roleSend)
					if err != nil {
						syncer.logf(logger.ERROR, "update beacon syncer %X ctrl send failed %s",
							ss.SenderGUID, err)
					}
				case protocol.Node:
					err = syncer.ctx.db.UpdateNSCtrlReceive(ss.SenderGUID, roleSend)
					if err != nil {
						syncer.logf(logger.ERROR, "update node syncer %X ctrl send failed %s",
							ss.SenderGUID, err)
					}
				}
				syncer.syncDone(ss.SenderRole, roleGUID)
				// notice node to delete message
				syncer.ctx.sender.SyncReceive(ss.SenderRole, ss.SenderGUID, roleSend-1)
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
				syncer.logf(logger.EXPLOIT, "invalid broadcast role: %s guid: %X",
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
		// case st = <-syncer.syncTaskQueue:
		// reply, err := query(sync_query)
		case <-syncer.stopSignal:
			return
		}
	}
}

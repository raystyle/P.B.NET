package node

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/xpanic"
)

// syncer is used to make sure every one message will
// be handle once, and start a cleaner to release memory.
type syncer struct {
	ctx *Node

	expireTime int64

	// value = timestamp
	// about controller
	sendToNodeGUID      map[guid.GUID]int64
	sendToNodeGUIDRWM   sync.RWMutex
	sendToBeaconGUID    map[guid.GUID]int64
	sendToBeaconGUIDRWM sync.RWMutex
	ackToNodeGUID       map[guid.GUID]int64
	ackToNodeGUIDRWM    sync.RWMutex
	ackToBeaconGUID     map[guid.GUID]int64
	ackToBeaconGUIDRWM  sync.RWMutex
	broadcastGUID       map[guid.GUID]int64
	broadcastGUIDRWM    sync.RWMutex
	answerGUID          map[guid.GUID]int64
	answerGUIDRWM       sync.RWMutex

	// about node
	nodeSendGUID    map[guid.GUID]int64
	nodeSendGUIDRWM sync.RWMutex
	nodeAckGUID     map[guid.GUID]int64
	nodeAckGUIDRWM  sync.RWMutex

	// about beacon
	beaconSendGUID    map[guid.GUID]int64
	beaconSendGUIDRWM sync.RWMutex
	beaconAckGUID     map[guid.GUID]int64
	beaconAckGUIDRWM  sync.RWMutex
	queryGUID         map[guid.GUID]int64
	queryGUIDRWM      sync.RWMutex

	// convert guid []byte to *guid.GUID
	mapKeyPool sync.Pool

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSyncer(ctx *Node, config *Config) (*syncer, error) {
	cfg := config.Syncer

	if cfg.ExpireTime < 3*time.Second || cfg.ExpireTime > 30*time.Second {
		return nil, errors.New("expire time < 3 seconds or > 30 seconds")
	}

	syncer := syncer{
		ctx:              ctx,
		expireTime:       int64(cfg.ExpireTime.Seconds()),
		sendToNodeGUID:   make(map[guid.GUID]int64),
		sendToBeaconGUID: make(map[guid.GUID]int64),
		ackToNodeGUID:    make(map[guid.GUID]int64),
		ackToBeaconGUID:  make(map[guid.GUID]int64),
		answerGUID:       make(map[guid.GUID]int64),
		broadcastGUID:    make(map[guid.GUID]int64),
		nodeSendGUID:     make(map[guid.GUID]int64),
		nodeAckGUID:      make(map[guid.GUID]int64),
		beaconSendGUID:   make(map[guid.GUID]int64),
		beaconAckGUID:    make(map[guid.GUID]int64),
		queryGUID:        make(map[guid.GUID]int64),
		stopSignal:       make(chan struct{}),
	}
	syncer.mapKeyPool.New = func() interface{} {
		return guid.GUID{}
	}
	syncer.wg.Add(1)
	go syncer.guidCleaner()
	return &syncer, nil
}

// CheckGUIDSliceTimestamp is used to check GUID is expire, parameter is []byte
func (syncer *syncer) CheckGUIDSliceTimestamp(guid []byte) bool {
	// look internal/guid/guid.go to understand guid[32:40]
	timestamp := convert.BytesToInt64(guid[32:40])
	now := syncer.ctx.global.Now().Unix()
	return convert.AbsInt64(now-timestamp) > syncer.expireTime
}

// CheckGUIDTimestamp is used to check GUID is expire
func (syncer *syncer) CheckGUIDTimestamp(guid *guid.GUID) (bool, int64) {
	now := syncer.ctx.global.Now().Unix()
	timestamp := guid.Timestamp()
	return convert.AbsInt64(now-timestamp) > syncer.expireTime, timestamp
}

// --------------------------code generated by resource/code/syncer.go-----------------------------

func (syncer *syncer) CheckSendToNodeGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.sendToNodeGUIDRWM.RLock()
	defer syncer.sendToNodeGUIDRWM.RUnlock()
	_, ok := syncer.sendToNodeGUID[key]
	return !ok
}

func (syncer *syncer) CheckSendToBeaconGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.sendToBeaconGUIDRWM.RLock()
	defer syncer.sendToBeaconGUIDRWM.RUnlock()
	_, ok := syncer.sendToBeaconGUID[key]
	return !ok
}

func (syncer *syncer) CheckAckToNodeGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.ackToNodeGUIDRWM.RLock()
	defer syncer.ackToNodeGUIDRWM.RUnlock()
	_, ok := syncer.ackToNodeGUID[key]
	return !ok
}

func (syncer *syncer) CheckAckToBeaconGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.ackToBeaconGUIDRWM.RLock()
	defer syncer.ackToBeaconGUIDRWM.RUnlock()
	_, ok := syncer.ackToBeaconGUID[key]
	return !ok
}

func (syncer *syncer) CheckBroadcastGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.broadcastGUIDRWM.RLock()
	defer syncer.broadcastGUIDRWM.RUnlock()
	_, ok := syncer.broadcastGUID[key]
	return !ok
}

func (syncer *syncer) CheckAnswerGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.answerGUIDRWM.RLock()
	defer syncer.answerGUIDRWM.RUnlock()
	_, ok := syncer.answerGUID[key]
	return !ok
}

func (syncer *syncer) CheckNodeSendGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.nodeSendGUIDRWM.RLock()
	defer syncer.nodeSendGUIDRWM.RUnlock()
	_, ok := syncer.nodeSendGUID[key]
	return !ok
}

func (syncer *syncer) CheckNodeAckGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.nodeAckGUIDRWM.RLock()
	defer syncer.nodeAckGUIDRWM.RUnlock()
	_, ok := syncer.nodeAckGUID[key]
	return !ok
}

func (syncer *syncer) CheckBeaconSendGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.beaconSendGUIDRWM.RLock()
	defer syncer.beaconSendGUIDRWM.RUnlock()
	_, ok := syncer.beaconSendGUID[key]
	return !ok
}

func (syncer *syncer) CheckBeaconAckGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.beaconAckGUIDRWM.RLock()
	defer syncer.beaconAckGUIDRWM.RUnlock()
	_, ok := syncer.beaconAckGUID[key]
	return !ok
}

func (syncer *syncer) CheckQueryGUIDSlice(slice []byte) bool {
	key := syncer.mapKeyPool.Get().(guid.GUID)
	defer syncer.mapKeyPool.Put(key)
	copy(key[:], slice)
	syncer.queryGUIDRWM.RLock()
	defer syncer.queryGUIDRWM.RUnlock()
	_, ok := syncer.queryGUID[key]
	return !ok
}

func (syncer *syncer) CheckSendToNodeGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.sendToNodeGUIDRWM.Lock()
	defer syncer.sendToNodeGUIDRWM.Unlock()
	if _, ok := syncer.sendToNodeGUID[*guid]; ok {
		return false
	}
	syncer.sendToNodeGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckSendToBeaconGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.sendToBeaconGUIDRWM.Lock()
	defer syncer.sendToBeaconGUIDRWM.Unlock()
	if _, ok := syncer.sendToBeaconGUID[*guid]; ok {
		return false
	}
	syncer.sendToBeaconGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckAckToNodeGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.ackToNodeGUIDRWM.Lock()
	defer syncer.ackToNodeGUIDRWM.Unlock()
	if _, ok := syncer.ackToNodeGUID[*guid]; ok {
		return false
	}
	syncer.ackToNodeGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckAckToBeaconGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.ackToBeaconGUIDRWM.Lock()
	defer syncer.ackToBeaconGUIDRWM.Unlock()
	if _, ok := syncer.ackToBeaconGUID[*guid]; ok {
		return false
	}
	syncer.ackToBeaconGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckBroadcastGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.broadcastGUIDRWM.Lock()
	defer syncer.broadcastGUIDRWM.Unlock()
	if _, ok := syncer.broadcastGUID[*guid]; ok {
		return false
	}
	syncer.broadcastGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckAnswerGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.answerGUIDRWM.Lock()
	defer syncer.answerGUIDRWM.Unlock()
	if _, ok := syncer.answerGUID[*guid]; ok {
		return false
	}
	syncer.answerGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckNodeSendGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.nodeSendGUIDRWM.Lock()
	defer syncer.nodeSendGUIDRWM.Unlock()
	if _, ok := syncer.nodeSendGUID[*guid]; ok {
		return false
	}
	syncer.nodeSendGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckNodeAckGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.nodeAckGUIDRWM.Lock()
	defer syncer.nodeAckGUIDRWM.Unlock()
	if _, ok := syncer.nodeAckGUID[*guid]; ok {
		return false
	}
	syncer.nodeAckGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckBeaconSendGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.beaconSendGUIDRWM.Lock()
	defer syncer.beaconSendGUIDRWM.Unlock()
	if _, ok := syncer.beaconSendGUID[*guid]; ok {
		return false
	}
	syncer.beaconSendGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckBeaconAckGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.beaconAckGUIDRWM.Lock()
	defer syncer.beaconAckGUIDRWM.Unlock()
	if _, ok := syncer.beaconAckGUID[*guid]; ok {
		return false
	}
	syncer.beaconAckGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) CheckQueryGUID(guid *guid.GUID, timestamp int64) bool {
	syncer.queryGUIDRWM.Lock()
	defer syncer.queryGUIDRWM.Unlock()
	if _, ok := syncer.queryGUID[*guid]; ok {
		return false
	}
	syncer.queryGUID[*guid] = timestamp
	return true
}

func (syncer *syncer) Close() {
	close(syncer.stopSignal)
	syncer.wg.Wait()
	syncer.ctx = nil
}

// guidCleaner is use to clean expire guid
func (syncer *syncer) guidCleaner() {
	defer func() {
		if r := recover(); r != nil {
			b := xpanic.Print(r, "syncer.guidCleaner")
			syncer.ctx.logger.Print(logger.Fatal, "syncer", b)
			// restart GUID cleaner
			time.Sleep(time.Second)
			go syncer.guidCleaner()
		} else {
			syncer.wg.Done()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	count := 0
	max := int(syncer.expireTime)
	for {
		select {
		case <-ticker.C:
			syncer.cleanGUID()
			count++
			if count > max {
				syncer.cleanGUIDMap()
				count = 0
			}
		case <-syncer.stopSignal:
			return
		}
	}
}

func (syncer *syncer) cleanGUID() {
	now := syncer.ctx.global.Now().Unix()

	syncer.cleanSendToNodeGUID(now)
	syncer.cleanSendToBeaconGUID(now)
	syncer.cleanAckToNodeGUID(now)
	syncer.cleanAckToBeaconGUID(now)
	syncer.cleanBroadcastGUID(now)
	syncer.cleanAnswerGUID(now)

	syncer.cleanNodeSendGUID(now)
	syncer.cleanNodeAckGUID(now)

	syncer.cleanBeaconSendGUID(now)
	syncer.cleanBeaconAckGUID(now)
	syncer.cleanQueryGUID(now)
}

func (syncer *syncer) cleanSendToNodeGUID(now int64) {
	syncer.sendToNodeGUIDRWM.Lock()
	defer syncer.sendToNodeGUIDRWM.Unlock()
	for key, timestamp := range syncer.sendToNodeGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.sendToNodeGUID, key)
		}
	}
}

func (syncer *syncer) cleanSendToBeaconGUID(now int64) {
	syncer.sendToBeaconGUIDRWM.Lock()
	defer syncer.sendToBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.sendToBeaconGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.sendToBeaconGUID, key)
		}
	}
}

func (syncer *syncer) cleanAckToNodeGUID(now int64) {
	syncer.ackToNodeGUIDRWM.Lock()
	defer syncer.ackToNodeGUIDRWM.Unlock()
	for key, timestamp := range syncer.ackToNodeGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.ackToNodeGUID, key)
		}
	}
}

func (syncer *syncer) cleanAckToBeaconGUID(now int64) {
	syncer.ackToBeaconGUIDRWM.Lock()
	defer syncer.ackToBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.ackToBeaconGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.ackToBeaconGUID, key)
		}
	}
}

func (syncer *syncer) cleanBroadcastGUID(now int64) {
	syncer.broadcastGUIDRWM.Lock()
	defer syncer.broadcastGUIDRWM.Unlock()
	for key, timestamp := range syncer.broadcastGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.broadcastGUID, key)
		}
	}
}

func (syncer *syncer) cleanAnswerGUID(now int64) {
	syncer.answerGUIDRWM.Lock()
	defer syncer.answerGUIDRWM.Unlock()
	for key, timestamp := range syncer.answerGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.answerGUID, key)
		}
	}
}

func (syncer *syncer) cleanNodeSendGUID(now int64) {
	syncer.nodeSendGUIDRWM.Lock()
	defer syncer.nodeSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeSendGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.nodeSendGUID, key)
		}
	}
}

func (syncer *syncer) cleanNodeAckGUID(now int64) {
	syncer.nodeAckGUIDRWM.Lock()
	defer syncer.nodeAckGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeAckGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.nodeAckGUID, key)
		}
	}
}

func (syncer *syncer) cleanBeaconSendGUID(now int64) {
	syncer.beaconSendGUIDRWM.Lock()
	defer syncer.beaconSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconSendGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.beaconSendGUID, key)
		}
	}
}

func (syncer *syncer) cleanBeaconAckGUID(now int64) {
	syncer.beaconAckGUIDRWM.Lock()
	defer syncer.beaconAckGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconAckGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.beaconAckGUID, key)
		}
	}
}

func (syncer *syncer) cleanQueryGUID(now int64) {
	syncer.queryGUIDRWM.Lock()
	defer syncer.queryGUIDRWM.Unlock()
	for key, timestamp := range syncer.queryGUID {
		if convert.AbsInt64(now-timestamp) > syncer.expireTime {
			delete(syncer.queryGUID, key)
		}
	}
}

func (syncer *syncer) cleanGUIDMap() {
	syncer.cleanSendToNodeGUIDMap()
	syncer.cleanSendToBeaconGUIDMap()
	syncer.cleanAckToNodeGUIDMap()
	syncer.cleanAckToBeaconGUIDMap()
	syncer.cleanBroadcastGUIDMap()
	syncer.cleanAnswerGUIDMap()

	syncer.cleanNodeSendGUIDMap()
	syncer.cleanNodeAckGUIDMap()

	syncer.cleanBeaconSendGUIDMap()
	syncer.cleanBeaconAckGUIDMap()
	syncer.cleanQueryGUIDMap()
}

func (syncer *syncer) cleanSendToNodeGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.sendToNodeGUIDRWM.Lock()
	defer syncer.sendToNodeGUIDRWM.Unlock()
	for key, timestamp := range syncer.sendToNodeGUID {
		newMap[key] = timestamp
	}
	syncer.sendToNodeGUID = newMap
}

func (syncer *syncer) cleanSendToBeaconGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.sendToBeaconGUIDRWM.Lock()
	defer syncer.sendToBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.sendToBeaconGUID {
		newMap[key] = timestamp
	}
	syncer.sendToBeaconGUID = newMap
}

func (syncer *syncer) cleanAckToNodeGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.ackToNodeGUIDRWM.Lock()
	defer syncer.ackToNodeGUIDRWM.Unlock()
	for key, timestamp := range syncer.ackToNodeGUID {
		newMap[key] = timestamp
	}
	syncer.ackToNodeGUID = newMap
}

func (syncer *syncer) cleanAckToBeaconGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.ackToBeaconGUIDRWM.Lock()
	defer syncer.ackToBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.ackToBeaconGUID {
		newMap[key] = timestamp
	}
	syncer.ackToBeaconGUID = newMap
}

func (syncer *syncer) cleanBroadcastGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.broadcastGUIDRWM.Lock()
	defer syncer.broadcastGUIDRWM.Unlock()
	for key, timestamp := range syncer.broadcastGUID {
		newMap[key] = timestamp
	}
	syncer.broadcastGUID = newMap
}

func (syncer *syncer) cleanAnswerGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.answerGUIDRWM.Lock()
	defer syncer.answerGUIDRWM.Unlock()
	for key, timestamp := range syncer.answerGUID {
		newMap[key] = timestamp
	}
	syncer.answerGUID = newMap
}

func (syncer *syncer) cleanNodeSendGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.nodeSendGUIDRWM.Lock()
	defer syncer.nodeSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeSendGUID {
		newMap[key] = timestamp
	}
	syncer.nodeSendGUID = newMap
}

func (syncer *syncer) cleanNodeAckGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.nodeAckGUIDRWM.Lock()
	defer syncer.nodeAckGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeAckGUID {
		newMap[key] = timestamp
	}
	syncer.nodeAckGUID = newMap
}

func (syncer *syncer) cleanBeaconSendGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.beaconSendGUIDRWM.Lock()
	defer syncer.beaconSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconSendGUID {
		newMap[key] = timestamp
	}
	syncer.beaconSendGUID = newMap
}

func (syncer *syncer) cleanBeaconAckGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.beaconAckGUIDRWM.Lock()
	defer syncer.beaconAckGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconAckGUID {
		newMap[key] = timestamp
	}
	syncer.beaconAckGUID = newMap
}

func (syncer *syncer) cleanQueryGUIDMap() {
	newMap := make(map[guid.GUID]int64)
	syncer.queryGUIDRWM.Lock()
	defer syncer.queryGUIDRWM.Unlock()
	for key, timestamp := range syncer.queryGUID {
		newMap[key] = timestamp
	}
	syncer.queryGUID = newMap
}

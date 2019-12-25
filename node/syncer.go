package node

import (
	"encoding/hex"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/xpanic"
)

// syncer is used to make sure every one message will
// be handle once, and start a cleaner to release memory
type syncer struct {
	ctx *Node

	expireTime float64

	// key = hex(GUID) value = timestamp
	// about controller
	sendToNodeGUID      map[string]int64
	sendToNodeGUIDRWM   sync.RWMutex
	sendToBeaconGUID    map[string]int64
	sendToBeaconGUIDRWM sync.RWMutex
	ackToNodeGUID       map[string]int64
	ackToNodeGUIDRWM    sync.RWMutex
	ackToBeaconGUID     map[string]int64
	ackToBeaconGUIDRWM  sync.RWMutex
	broadcastGUID       map[string]int64
	broadcastGUIDRWM    sync.RWMutex
	answerGUID          map[string]int64
	answerGUIDRWM       sync.RWMutex

	// about node
	nodeSendGUID    map[string]int64
	nodeSendGUIDRWM sync.RWMutex
	nodeAckGUID     map[string]int64
	nodeAckGUIDRWM  sync.RWMutex

	// about beacon
	beaconSendGUID    map[string]int64
	beaconSendGUIDRWM sync.RWMutex
	beaconAckGUID     map[string]int64
	beaconAckGUIDRWM  sync.RWMutex
	queryGUID         map[string]int64
	queryGUIDRWM      sync.RWMutex

	// calculate key
	hexPool sync.Pool

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
		expireTime:       cfg.ExpireTime.Seconds(),
		sendToNodeGUID:   make(map[string]int64),
		sendToBeaconGUID: make(map[string]int64),
		ackToNodeGUID:    make(map[string]int64),
		ackToBeaconGUID:  make(map[string]int64),
		answerGUID:       make(map[string]int64),
		broadcastGUID:    make(map[string]int64),
		nodeSendGUID:     make(map[string]int64),
		nodeAckGUID:      make(map[string]int64),
		beaconSendGUID:   make(map[string]int64),
		beaconAckGUID:    make(map[string]int64),
		queryGUID:        make(map[string]int64),
		stopSignal:       make(chan struct{}),
	}
	syncer.hexPool.New = func() interface{} {
		return make([]byte, 2*guid.Size)
	}
	syncer.wg.Add(1)
	go syncer.guidCleaner()
	return &syncer, nil
}

// CheckGUIDTimestamp is used to get timestamp from GUID
func (syncer *syncer) CheckGUIDTimestamp(guid []byte) (bool, int64) {
	// look internal/guid/guid.go
	timestamp := convert.BytesToInt64(guid[36:44])
	now := syncer.ctx.global.Now().Unix()
	if math.Abs(float64(now-timestamp)) > syncer.expireTime {
		return true, 0
	}
	return false, timestamp
}

func (syncer *syncer) calculateKey(guid []byte) string {
	dst := syncer.hexPool.Get().([]byte)
	defer syncer.hexPool.Put(dst)
	hex.Encode(dst, guid)
	return string(dst)
}

func (syncer *syncer) CheckSendToNodeGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.sendToNodeGUIDRWM.Lock()
		defer syncer.sendToNodeGUIDRWM.Unlock()
		if _, ok := syncer.sendToNodeGUID[key]; ok {
			return false
		}
		syncer.sendToNodeGUID[key] = timestamp
		return true
	}
	syncer.sendToNodeGUIDRWM.RLock()
	defer syncer.sendToNodeGUIDRWM.RUnlock()
	_, ok := syncer.sendToNodeGUID[key]
	return !ok
}

func (syncer *syncer) CheckSendToBeaconGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.sendToBeaconGUIDRWM.Lock()
		defer syncer.sendToBeaconGUIDRWM.Unlock()
		if _, ok := syncer.sendToBeaconGUID[key]; ok {
			return false
		}
		syncer.sendToBeaconGUID[key] = timestamp
		return true
	}
	syncer.sendToBeaconGUIDRWM.RLock()
	defer syncer.sendToBeaconGUIDRWM.RUnlock()
	_, ok := syncer.sendToBeaconGUID[key]
	return !ok
}

func (syncer *syncer) CheckAckToNodeGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.ackToNodeGUIDRWM.Lock()
		defer syncer.ackToNodeGUIDRWM.Unlock()
		if _, ok := syncer.ackToNodeGUID[key]; ok {
			return false
		}
		syncer.ackToNodeGUID[key] = timestamp
		return true
	}
	syncer.ackToNodeGUIDRWM.RLock()
	defer syncer.ackToNodeGUIDRWM.RUnlock()
	_, ok := syncer.ackToNodeGUID[key]
	return !ok
}

func (syncer *syncer) CheckAckToBeaconGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.ackToBeaconGUIDRWM.Lock()
		defer syncer.ackToBeaconGUIDRWM.Unlock()
		if _, ok := syncer.ackToBeaconGUID[key]; ok {
			return false
		}
		syncer.ackToBeaconGUID[key] = timestamp
		return true
	}
	syncer.ackToBeaconGUIDRWM.RLock()
	defer syncer.ackToBeaconGUIDRWM.RUnlock()
	_, ok := syncer.ackToBeaconGUID[key]
	return !ok
}

func (syncer *syncer) CheckBroadcastGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.broadcastGUIDRWM.Lock()
		defer syncer.broadcastGUIDRWM.Unlock()
		if _, ok := syncer.broadcastGUID[key]; ok {
			return false
		}
		syncer.broadcastGUID[key] = timestamp
		return true
	}
	syncer.broadcastGUIDRWM.RLock()
	defer syncer.broadcastGUIDRWM.RUnlock()
	_, ok := syncer.broadcastGUID[key]
	return !ok
}

func (syncer *syncer) CheckAnswerGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.answerGUIDRWM.Lock()
		defer syncer.answerGUIDRWM.Unlock()
		if _, ok := syncer.answerGUID[key]; ok {
			return false
		}
		syncer.answerGUID[key] = timestamp
		return true
	}
	syncer.answerGUIDRWM.RLock()
	defer syncer.answerGUIDRWM.RUnlock()
	_, ok := syncer.answerGUID[key]
	return !ok
}

func (syncer *syncer) CheckNodeSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.nodeSendGUIDRWM.Lock()
		defer syncer.nodeSendGUIDRWM.Unlock()
		if _, ok := syncer.nodeSendGUID[key]; ok {
			return false
		}
		syncer.nodeSendGUID[key] = timestamp
		return true
	}
	syncer.nodeSendGUIDRWM.RLock()
	defer syncer.nodeSendGUIDRWM.RUnlock()
	_, ok := syncer.nodeSendGUID[key]
	return !ok
}

func (syncer *syncer) CheckNodeAckGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.nodeAckGUIDRWM.Lock()
		defer syncer.nodeAckGUIDRWM.Unlock()
		if _, ok := syncer.nodeAckGUID[key]; ok {
			return false
		}
		syncer.nodeAckGUID[key] = timestamp
		return true
	}
	syncer.nodeAckGUIDRWM.RLock()
	defer syncer.nodeAckGUIDRWM.RUnlock()
	_, ok := syncer.nodeAckGUID[key]
	return !ok
}

func (syncer *syncer) CheckBeaconSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.beaconSendGUIDRWM.Lock()
		defer syncer.beaconSendGUIDRWM.Unlock()
		if _, ok := syncer.beaconSendGUID[key]; ok {
			return false
		}
		syncer.beaconSendGUID[key] = timestamp
		return true
	}
	syncer.beaconSendGUIDRWM.RLock()
	defer syncer.beaconSendGUIDRWM.RUnlock()
	_, ok := syncer.beaconSendGUID[key]
	return !ok
}

func (syncer *syncer) CheckBeaconAckGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.beaconAckGUIDRWM.Lock()
		defer syncer.beaconAckGUIDRWM.Unlock()
		if _, ok := syncer.beaconAckGUID[key]; ok {
			return false
		}
		syncer.beaconAckGUID[key] = timestamp
		return true
	}
	syncer.beaconAckGUIDRWM.RLock()
	defer syncer.beaconAckGUIDRWM.RUnlock()
	_, ok := syncer.beaconAckGUID[key]
	return !ok
}

func (syncer *syncer) CheckQueryGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.queryGUIDRWM.Lock()
		defer syncer.queryGUIDRWM.Unlock()
		if _, ok := syncer.queryGUID[key]; ok {
			return false
		}
		syncer.queryGUID[key] = timestamp
		return true
	}
	syncer.queryGUIDRWM.RLock()
	defer syncer.queryGUIDRWM.RUnlock()
	_, ok := syncer.queryGUID[key]
	return !ok
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
			err := xpanic.Error(r, "syncer.guidCleaner")
			syncer.ctx.logger.Print(logger.Fatal, "syncer", err)
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
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.sendToNodeGUID, key)
		}
	}
}

func (syncer *syncer) cleanSendToBeaconGUID(now int64) {
	syncer.sendToBeaconGUIDRWM.Lock()
	defer syncer.sendToBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.sendToBeaconGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.sendToBeaconGUID, key)
		}
	}
}

func (syncer *syncer) cleanAckToNodeGUID(now int64) {
	syncer.ackToNodeGUIDRWM.Lock()
	defer syncer.ackToNodeGUIDRWM.Unlock()
	for key, timestamp := range syncer.ackToNodeGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ackToNodeGUID, key)
		}
	}
}

func (syncer *syncer) cleanAckToBeaconGUID(now int64) {
	syncer.ackToBeaconGUIDRWM.Lock()
	defer syncer.ackToBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.ackToBeaconGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ackToBeaconGUID, key)
		}
	}
}

func (syncer *syncer) cleanBroadcastGUID(now int64) {
	syncer.broadcastGUIDRWM.Lock()
	defer syncer.broadcastGUIDRWM.Unlock()
	for key, timestamp := range syncer.broadcastGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.broadcastGUID, key)
		}
	}
}

func (syncer *syncer) cleanAnswerGUID(now int64) {
	syncer.answerGUIDRWM.Lock()
	defer syncer.answerGUIDRWM.Unlock()
	for key, timestamp := range syncer.answerGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.answerGUID, key)
		}
	}
}

func (syncer *syncer) cleanNodeSendGUID(now int64) {
	syncer.nodeSendGUIDRWM.Lock()
	defer syncer.nodeSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeSendGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.nodeSendGUID, key)
		}
	}
}

func (syncer *syncer) cleanNodeAckGUID(now int64) {
	syncer.nodeAckGUIDRWM.Lock()
	defer syncer.nodeAckGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeAckGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.nodeAckGUID, key)
		}
	}
}

func (syncer *syncer) cleanBeaconSendGUID(now int64) {
	syncer.beaconSendGUIDRWM.Lock()
	defer syncer.beaconSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconSendGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.beaconSendGUID, key)
		}
	}
}

func (syncer *syncer) cleanBeaconAckGUID(now int64) {
	syncer.beaconAckGUIDRWM.Lock()
	defer syncer.beaconAckGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconAckGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.beaconAckGUID, key)
		}
	}
}

func (syncer *syncer) cleanQueryGUID(now int64) {
	syncer.queryGUIDRWM.Lock()
	defer syncer.queryGUIDRWM.Unlock()
	for key, timestamp := range syncer.queryGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
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
	syncer.sendToNodeGUIDRWM.Lock()
	defer syncer.sendToNodeGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.sendToNodeGUID {
		newMap[key] = timestamp
	}
	syncer.sendToNodeGUID = newMap
}

func (syncer *syncer) cleanSendToBeaconGUIDMap() {
	syncer.sendToBeaconGUIDRWM.Lock()
	defer syncer.sendToBeaconGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.sendToBeaconGUID {
		newMap[key] = timestamp
	}
	syncer.sendToBeaconGUID = newMap
}

func (syncer *syncer) cleanAckToNodeGUIDMap() {
	syncer.ackToNodeGUIDRWM.Lock()
	defer syncer.ackToNodeGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ackToNodeGUID {
		newMap[key] = timestamp
	}
	syncer.ackToNodeGUID = newMap
}

func (syncer *syncer) cleanAckToBeaconGUIDMap() {
	syncer.ackToBeaconGUIDRWM.Lock()
	defer syncer.ackToBeaconGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ackToBeaconGUID {
		newMap[key] = timestamp
	}
	syncer.ackToBeaconGUID = newMap
}

func (syncer *syncer) cleanBroadcastGUIDMap() {
	syncer.broadcastGUIDRWM.Lock()
	defer syncer.broadcastGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.broadcastGUID {
		newMap[key] = timestamp
	}
	syncer.broadcastGUID = newMap
}

func (syncer *syncer) cleanAnswerGUIDMap() {
	syncer.answerGUIDRWM.Lock()
	defer syncer.answerGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.answerGUID {
		newMap[key] = timestamp
	}
	syncer.answerGUID = newMap
}

func (syncer *syncer) cleanNodeSendGUIDMap() {
	syncer.nodeSendGUIDRWM.Lock()
	defer syncer.nodeSendGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.nodeSendGUID {
		newMap[key] = timestamp
	}
	syncer.nodeSendGUID = newMap
}

func (syncer *syncer) cleanNodeAckGUIDMap() {
	syncer.nodeAckGUIDRWM.Lock()
	defer syncer.nodeAckGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.nodeAckGUID {
		newMap[key] = timestamp
	}
	syncer.nodeAckGUID = newMap
}

func (syncer *syncer) cleanBeaconSendGUIDMap() {
	syncer.beaconSendGUIDRWM.Lock()
	defer syncer.beaconSendGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.beaconSendGUID {
		newMap[key] = timestamp
	}
	syncer.beaconSendGUID = newMap
}

func (syncer *syncer) cleanBeaconAckGUIDMap() {
	syncer.beaconAckGUIDRWM.Lock()
	defer syncer.beaconAckGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.beaconAckGUID {
		newMap[key] = timestamp
	}
	syncer.beaconAckGUID = newMap
}

func (syncer *syncer) cleanQueryGUIDMap() {
	syncer.queryGUIDRWM.Lock()
	defer syncer.queryGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.queryGUID {
		newMap[key] = timestamp
	}
	syncer.queryGUID = newMap
}

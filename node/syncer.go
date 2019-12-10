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
	// controller send and broadcast
	ctrlSendGUID           map[string]int64
	ctrlSendGUIDRWM        sync.RWMutex
	ctrlAckToNodeGUID      map[string]int64
	ctrlAckToNodeGUIDRWM   sync.RWMutex
	ctrlAckToBeaconGUID    map[string]int64
	ctrlAckToBeaconGUIDRWM sync.RWMutex
	broadcastGUID          map[string]int64
	broadcastGUIDRWM       sync.RWMutex
	answerGUID             map[string]int64
	answerGUIDRWM          sync.RWMutex

	// node send
	nodeSendGUID         map[string]int64
	nodeSendGUIDRWM      sync.RWMutex
	nodeAckToCtrlGUID    map[string]int64
	nodeAckToCtrlGUIDRWM sync.RWMutex

	// beacon send and query
	beaconSendGUID         map[string]int64
	beaconSendGUIDRWM      sync.RWMutex
	beaconAckToCtrlGUID    map[string]int64
	beaconAckToCtrlGUIDRWM sync.RWMutex
	queryGUID              map[string]int64
	queryGUIDRWM           sync.RWMutex

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
		ctx:                 ctx,
		expireTime:          cfg.ExpireTime.Seconds(),
		ctrlSendGUID:        make(map[string]int64),
		ctrlAckToNodeGUID:   make(map[string]int64),
		ctrlAckToBeaconGUID: make(map[string]int64),
		answerGUID:          make(map[string]int64),
		broadcastGUID:       make(map[string]int64),
		nodeSendGUID:        make(map[string]int64),
		nodeAckToCtrlGUID:   make(map[string]int64),
		beaconSendGUID:      make(map[string]int64),
		beaconAckToCtrlGUID: make(map[string]int64),
		queryGUID:           make(map[string]int64),
		stopSignal:          make(chan struct{}),
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

func (syncer *syncer) CheckCtrlSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.ctrlSendGUIDRWM.Lock()
		defer syncer.ctrlSendGUIDRWM.Unlock()
		if _, ok := syncer.ctrlSendGUID[key]; ok {
			return false
		}
		syncer.ctrlSendGUID[key] = timestamp
		return true
	}
	syncer.ctrlSendGUIDRWM.RLock()
	defer syncer.ctrlSendGUIDRWM.RUnlock()
	_, ok := syncer.ctrlSendGUID[key]
	return !ok
}

func (syncer *syncer) CheckCtrlAckToNodeGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.ctrlAckToNodeGUIDRWM.Lock()
		defer syncer.ctrlAckToNodeGUIDRWM.Unlock()
		if _, ok := syncer.ctrlAckToNodeGUID[key]; ok {
			return false
		}
		syncer.ctrlAckToNodeGUID[key] = timestamp
		return true
	}
	syncer.ctrlAckToNodeGUIDRWM.RLock()
	defer syncer.ctrlAckToNodeGUIDRWM.RUnlock()
	_, ok := syncer.ctrlAckToNodeGUID[key]
	return !ok
}

func (syncer *syncer) CheckCtrlAckToBeaconGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.ctrlAckToBeaconGUIDRWM.Lock()
		defer syncer.ctrlAckToBeaconGUIDRWM.Unlock()
		if _, ok := syncer.ctrlAckToBeaconGUID[key]; ok {
			return false
		}
		syncer.ctrlAckToBeaconGUID[key] = timestamp
		return true
	}
	syncer.ctrlAckToBeaconGUIDRWM.RLock()
	defer syncer.ctrlAckToBeaconGUIDRWM.RUnlock()
	_, ok := syncer.ctrlAckToBeaconGUID[key]
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

func (syncer *syncer) CheckNodeAckToCtrlGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.nodeAckToCtrlGUIDRWM.Lock()
		defer syncer.nodeAckToCtrlGUIDRWM.Unlock()
		if _, ok := syncer.nodeAckToCtrlGUID[key]; ok {
			return false
		}
		syncer.nodeAckToCtrlGUID[key] = timestamp
		return true
	}
	syncer.nodeAckToCtrlGUIDRWM.RLock()
	defer syncer.nodeAckToCtrlGUIDRWM.RUnlock()
	_, ok := syncer.nodeAckToCtrlGUID[key]
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

func (syncer *syncer) CheckBeaconAckToCtrlGUID(guid []byte, add bool, timestamp int64) bool {
	key := syncer.calculateKey(guid)
	if add {
		syncer.beaconAckToCtrlGUIDRWM.Lock()
		defer syncer.beaconAckToCtrlGUIDRWM.Unlock()
		if _, ok := syncer.beaconAckToCtrlGUID[key]; ok {
			return false
		}
		syncer.beaconAckToCtrlGUID[key] = timestamp
		return true
	}
	syncer.beaconAckToCtrlGUIDRWM.RLock()
	defer syncer.beaconAckToCtrlGUIDRWM.RUnlock()
	_, ok := syncer.beaconAckToCtrlGUID[key]
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
			count += 1
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

	syncer.cleanCtrlSendGUID(now)
	syncer.cleanCtrlAckToNodeGUID(now)
	syncer.cleanCtrlAckToBeaconGUID(now)
	syncer.cleanBroadcastGUID(now)
	syncer.cleanAnswerGUID(now)

	syncer.cleanNodeSendGUID(now)
	syncer.cleanNodeAckToCtrlGUID(now)

	syncer.cleanBeaconSendGUID(now)
	syncer.cleanBeaconAckToCtrlGUID(now)
	syncer.cleanQueryGUID(now)
}

func (syncer *syncer) cleanCtrlSendGUID(now int64) {
	syncer.ctrlSendGUIDRWM.Lock()
	defer syncer.ctrlSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.ctrlSendGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ctrlSendGUID, key)
		}
	}
}

func (syncer *syncer) cleanCtrlAckToNodeGUID(now int64) {
	syncer.ctrlAckToNodeGUIDRWM.Lock()
	defer syncer.ctrlAckToNodeGUIDRWM.Unlock()
	for key, timestamp := range syncer.ctrlAckToNodeGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ctrlAckToNodeGUID, key)
		}
	}
}

func (syncer *syncer) cleanCtrlAckToBeaconGUID(now int64) {
	syncer.ctrlAckToBeaconGUIDRWM.Lock()
	defer syncer.ctrlAckToBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.ctrlAckToBeaconGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ctrlAckToBeaconGUID, key)
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

func (syncer *syncer) cleanNodeAckToCtrlGUID(now int64) {
	syncer.nodeAckToCtrlGUIDRWM.Lock()
	defer syncer.nodeAckToCtrlGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeAckToCtrlGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.nodeAckToCtrlGUID, key)
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

func (syncer *syncer) cleanBeaconAckToCtrlGUID(now int64) {
	syncer.beaconAckToCtrlGUIDRWM.Lock()
	defer syncer.beaconAckToCtrlGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconAckToCtrlGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.beaconAckToCtrlGUID, key)
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
	syncer.cleanCtrlSendGUIDMap()
	syncer.cleanCtrlAckToNodeGUIDMap()
	syncer.cleanCtrlAckToBeaconGUIDMap()
	syncer.cleanBroadcastGUIDMap()
	syncer.cleanAnswerGUIDMap()

	syncer.cleanNodeSendGUIDMap()
	syncer.cleanNodeAckToCtrlGUIDMap()

	syncer.cleanBeaconSendGUIDMap()
	syncer.cleanBeaconAckToCtrlGUIDMap()
	syncer.cleanQueryGUIDMap()
}

func (syncer *syncer) cleanCtrlSendGUIDMap() {
	syncer.ctrlSendGUIDRWM.Lock()
	defer syncer.ctrlSendGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ctrlSendGUID {
		newMap[key] = timestamp
	}
	syncer.ctrlSendGUID = newMap
}

func (syncer *syncer) cleanCtrlAckToNodeGUIDMap() {
	syncer.ctrlAckToNodeGUIDRWM.Lock()
	defer syncer.ctrlAckToNodeGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ctrlAckToNodeGUID {
		newMap[key] = timestamp
	}
	syncer.ctrlAckToNodeGUID = newMap
}

func (syncer *syncer) cleanCtrlAckToBeaconGUIDMap() {
	syncer.ctrlAckToBeaconGUIDRWM.Lock()
	defer syncer.ctrlAckToBeaconGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ctrlAckToBeaconGUID {
		newMap[key] = timestamp
	}
	syncer.ctrlAckToBeaconGUID = newMap
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

func (syncer *syncer) cleanNodeAckToCtrlGUIDMap() {
	syncer.nodeAckToCtrlGUIDRWM.Lock()
	defer syncer.nodeAckToCtrlGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.nodeAckToCtrlGUID {
		newMap[key] = timestamp
	}
	syncer.nodeAckToCtrlGUID = newMap
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

func (syncer *syncer) cleanBeaconAckToCtrlGUIDMap() {
	syncer.beaconAckToCtrlGUIDRWM.Lock()
	defer syncer.beaconAckToCtrlGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.beaconAckToCtrlGUID {
		newMap[key] = timestamp
	}
	syncer.beaconAckToCtrlGUID = newMap
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

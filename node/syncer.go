package node

import (
	"encoding/base64"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/xpanic"
)

// syncer is used to make sure every one message will
// be handle once, and start a cleaner to release memory
type syncer struct {
	ctx *Node

	expireTime float64

	// key = base64(GUID) value = timestamp
	// controller send and broadcast
	ctrlSendGUID         map[string]int64
	ctrlSendGUIDRWM      sync.RWMutex
	nodeAckCtrlGUID      map[string]int64
	nodeAckCtrlGUIDRWM   sync.RWMutex
	beaconAckCtrlGUID    map[string]int64
	beaconAckCtrlGUIDRWM sync.RWMutex
	broadcastGUID        map[string]int64
	broadcastGUIDRWM     sync.RWMutex

	// node send
	nodeSendGUID       map[string]int64
	nodeSendGUIDRWM    sync.RWMutex
	ctrlAckNodeGUID    map[string]int64
	ctrlAckNodeGUIDRWM sync.RWMutex

	// beacon send and query
	beaconSendGUID       map[string]int64
	beaconSendGUIDRWM    sync.RWMutex
	ctrlAckBeaconGUID    map[string]int64
	ctrlAckBeaconGUIDRWM sync.RWMutex
	beaconQueryGUID      map[string]int64
	beaconQueryGUIDRWM   sync.RWMutex
	ctrlAnswerGUID       map[string]int64
	ctrlAnswerGUIDRWM    sync.RWMutex

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newSyncer(ctx *Node, config *Config) (*syncer, error) {
	cfg := config.Syncer

	if cfg.ExpireTime < 5*time.Minute || cfg.ExpireTime > time.Hour {
		return nil, errors.New("expire time < 5m or > 1h")
	}

	syncer := syncer{
		ctx: ctx,

		expireTime: cfg.ExpireTime.Seconds(),

		ctrlSendGUID:      make(map[string]int64),
		nodeAckCtrlGUID:   make(map[string]int64),
		beaconAckCtrlGUID: make(map[string]int64),
		broadcastGUID:     make(map[string]int64),

		nodeSendGUID:    make(map[string]int64),
		ctrlAckNodeGUID: make(map[string]int64),

		beaconSendGUID:    make(map[string]int64),
		ctrlAckBeaconGUID: make(map[string]int64),
		beaconQueryGUID:   make(map[string]int64),
		ctrlAnswerGUID:    make(map[string]int64),

		stopSignal: make(chan struct{}),
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
		return false, 0
	}
	return true, timestamp
}

func (syncer *syncer) CheckCtrlSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.ctrlSendGUIDRWM.Lock()
		defer syncer.ctrlSendGUIDRWM.Unlock()
		if _, ok := syncer.ctrlSendGUID[key]; ok {
			return false
		} else {
			syncer.ctrlSendGUID[key] = timestamp
			return true
		}
	} else {
		syncer.ctrlSendGUIDRWM.RLock()
		defer syncer.ctrlSendGUIDRWM.RUnlock()
		_, ok := syncer.ctrlSendGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckNodeAckCtrlGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.nodeAckCtrlGUIDRWM.Lock()
		defer syncer.nodeAckCtrlGUIDRWM.Unlock()
		if _, ok := syncer.nodeAckCtrlGUID[key]; ok {
			return false
		} else {
			syncer.nodeAckCtrlGUID[key] = timestamp
			return true
		}
	} else {
		syncer.nodeAckCtrlGUIDRWM.RLock()
		defer syncer.nodeAckCtrlGUIDRWM.RUnlock()
		_, ok := syncer.nodeAckCtrlGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckBeaconAckCtrlGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.beaconAckCtrlGUIDRWM.Lock()
		defer syncer.beaconAckCtrlGUIDRWM.Unlock()
		if _, ok := syncer.beaconAckCtrlGUID[key]; ok {
			return false
		} else {
			syncer.beaconAckCtrlGUID[key] = timestamp
			return true
		}
	} else {
		syncer.beaconAckCtrlGUIDRWM.RLock()
		defer syncer.beaconAckCtrlGUIDRWM.RUnlock()
		_, ok := syncer.beaconAckCtrlGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckBroadcastGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.broadcastGUIDRWM.Lock()
		defer syncer.broadcastGUIDRWM.Unlock()
		if _, ok := syncer.broadcastGUID[key]; ok {
			return false
		} else {
			syncer.broadcastGUID[key] = timestamp
			return true
		}
	} else {
		syncer.broadcastGUIDRWM.RLock()
		defer syncer.broadcastGUIDRWM.RUnlock()
		_, ok := syncer.broadcastGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckNodeSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.nodeSendGUIDRWM.Lock()
		defer syncer.nodeSendGUIDRWM.Unlock()
		if _, ok := syncer.nodeSendGUID[key]; ok {
			return false
		} else {
			syncer.nodeSendGUID[key] = timestamp
			return true
		}
	} else {
		syncer.nodeSendGUIDRWM.RLock()
		defer syncer.nodeSendGUIDRWM.RUnlock()
		_, ok := syncer.nodeSendGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckCtrlAckNodeGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.ctrlAckNodeGUIDRWM.Lock()
		defer syncer.ctrlAckNodeGUIDRWM.Unlock()
		if _, ok := syncer.ctrlAckNodeGUID[key]; ok {
			return false
		} else {
			syncer.ctrlAckNodeGUID[key] = timestamp
			return true
		}
	} else {
		syncer.ctrlAckNodeGUIDRWM.RLock()
		defer syncer.ctrlAckNodeGUIDRWM.RUnlock()
		_, ok := syncer.ctrlAckNodeGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckBeaconSendGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.beaconSendGUIDRWM.Lock()
		defer syncer.beaconSendGUIDRWM.Unlock()
		if _, ok := syncer.beaconSendGUID[key]; ok {
			return false
		} else {
			syncer.beaconSendGUID[key] = timestamp
			return true
		}
	} else {
		syncer.beaconSendGUIDRWM.RLock()
		defer syncer.beaconSendGUIDRWM.RUnlock()
		_, ok := syncer.beaconSendGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckCtrlAckBeaconGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.ctrlAckBeaconGUIDRWM.Lock()
		defer syncer.ctrlAckBeaconGUIDRWM.Unlock()
		if _, ok := syncer.ctrlAckBeaconGUID[key]; ok {
			return false
		} else {
			syncer.ctrlAckBeaconGUID[key] = timestamp
			return true
		}
	} else {
		syncer.ctrlAckBeaconGUIDRWM.RLock()
		defer syncer.ctrlAckBeaconGUIDRWM.RUnlock()
		_, ok := syncer.ctrlAckBeaconGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckBeaconQueryGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.beaconQueryGUIDRWM.Lock()
		defer syncer.beaconQueryGUIDRWM.Unlock()
		if _, ok := syncer.beaconQueryGUID[key]; ok {
			return false
		} else {
			syncer.beaconQueryGUID[key] = timestamp
			return true
		}
	} else {
		syncer.beaconQueryGUIDRWM.RLock()
		defer syncer.beaconQueryGUIDRWM.RUnlock()
		_, ok := syncer.beaconQueryGUID[key]
		return !ok
	}
}

func (syncer *syncer) CheckCtrlAnswerGUID(guid []byte, add bool, timestamp int64) bool {
	key := base64.StdEncoding.EncodeToString(guid)
	if add {
		syncer.ctrlAnswerGUIDRWM.Lock()
		defer syncer.ctrlAnswerGUIDRWM.Unlock()
		if _, ok := syncer.ctrlAnswerGUID[key]; ok {
			return false
		} else {
			syncer.ctrlAnswerGUID[key] = timestamp
			return true
		}
	} else {
		syncer.ctrlAnswerGUIDRWM.RLock()
		defer syncer.ctrlAnswerGUIDRWM.RUnlock()
		_, ok := syncer.ctrlAnswerGUID[key]
		return !ok
	}
}

// guidCleaner is use to clean expire guid
func (syncer *syncer) guidCleaner() {
	defer func() {
		if r := recover(); r != nil {
			syncer.log(logger.Fatal, xpanic.Error(r, "syncer.guidCleaner"))
			// restart GUID cleaner
			time.Sleep(time.Second)
			go syncer.guidCleaner()
		} else {
			syncer.wg.Done()
		}
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	count := 0
	for {
		select {
		case <-ticker.C:
			syncer.cleanGUID()
			count += 1
			if count > 20 {
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
	syncer.cleanNodeAckCtrlGUID(now)
	syncer.cleanBeaconAckCtrlGUID(now)
	syncer.cleanBroadcastGUID(now)

	syncer.cleanNodeSendGUID(now)
	syncer.cleanCtrlAckNodeGUID(now)

	syncer.cleanBeaconSendGUID(now)
	syncer.cleanCtrlAckBeaconGUID(now)
	syncer.cleanBeaconQueryGUID(now)
	syncer.cleanCtrlAnswerGUID(now)

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

func (syncer *syncer) cleanNodeAckCtrlGUID(now int64) {
	syncer.nodeAckCtrlGUIDRWM.Lock()
	defer syncer.nodeAckCtrlGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeAckCtrlGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.nodeAckCtrlGUID, key)
		}
	}
}

func (syncer *syncer) cleanBeaconAckCtrlGUID(now int64) {
	syncer.beaconAckCtrlGUIDRWM.Lock()
	defer syncer.beaconAckCtrlGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconAckCtrlGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.beaconAckCtrlGUID, key)
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

func (syncer *syncer) cleanNodeSendGUID(now int64) {
	syncer.nodeSendGUIDRWM.Lock()
	defer syncer.nodeSendGUIDRWM.Unlock()
	for key, timestamp := range syncer.nodeSendGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.nodeSendGUID, key)
		}
	}
}

func (syncer *syncer) cleanCtrlAckNodeGUID(now int64) {
	syncer.ctrlAckNodeGUIDRWM.Lock()
	defer syncer.ctrlAckNodeGUIDRWM.Unlock()
	for key, timestamp := range syncer.ctrlAckNodeGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ctrlAckNodeGUID, key)
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

func (syncer *syncer) cleanCtrlAckBeaconGUID(now int64) {
	syncer.ctrlAckBeaconGUIDRWM.Lock()
	defer syncer.ctrlAckBeaconGUIDRWM.Unlock()
	for key, timestamp := range syncer.ctrlAckBeaconGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ctrlAckBeaconGUID, key)
		}
	}
}

func (syncer *syncer) cleanBeaconQueryGUID(now int64) {
	syncer.beaconQueryGUIDRWM.Lock()
	defer syncer.beaconQueryGUIDRWM.Unlock()
	for key, timestamp := range syncer.beaconQueryGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.beaconQueryGUID, key)
		}
	}
}

func (syncer *syncer) cleanCtrlAnswerGUID(now int64) {
	syncer.ctrlAnswerGUIDRWM.Lock()
	defer syncer.ctrlAnswerGUIDRWM.Unlock()
	for key, timestamp := range syncer.ctrlAnswerGUID {
		if math.Abs(float64(now-timestamp)) > syncer.expireTime {
			delete(syncer.ctrlAnswerGUID, key)
		}
	}
}

func (syncer *syncer) cleanGUIDMap() {
	syncer.cleanCtrlSendGUIDMap()
	syncer.cleanNodeAckCtrlGUIDMap()
	syncer.cleanBeaconAckCtrlGUIDMap()
	syncer.cleanBroadcastGUIDMap()

	syncer.cleanNodeSendGUIDMap()
	syncer.cleanCtrlAckNodeGUIDMap()

	syncer.cleanBeaconSendGUIDMap()
	syncer.cleanCtrlAckBeaconGUIDMap()
	syncer.cleanBeaconQueryGUIDMap()
	syncer.cleanCtrlAnswerGUIDMap()
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

func (syncer *syncer) cleanNodeAckCtrlGUIDMap() {
	syncer.nodeAckCtrlGUIDRWM.Lock()
	defer syncer.nodeAckCtrlGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.nodeAckCtrlGUID {
		newMap[key] = timestamp
	}
	syncer.nodeAckCtrlGUID = newMap
}

func (syncer *syncer) cleanBeaconAckCtrlGUIDMap() {
	syncer.beaconAckCtrlGUIDRWM.Lock()
	defer syncer.beaconAckCtrlGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.beaconAckCtrlGUID {
		newMap[key] = timestamp
	}
	syncer.beaconAckCtrlGUID = newMap
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

func (syncer *syncer) cleanNodeSendGUIDMap() {
	syncer.nodeSendGUIDRWM.Lock()
	defer syncer.nodeSendGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.nodeSendGUID {
		newMap[key] = timestamp
	}
	syncer.nodeSendGUID = newMap
}

func (syncer *syncer) cleanCtrlAckNodeGUIDMap() {
	syncer.ctrlAckNodeGUIDRWM.Lock()
	defer syncer.ctrlAckNodeGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ctrlAckNodeGUID {
		newMap[key] = timestamp
	}
	syncer.ctrlAckNodeGUID = newMap
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

func (syncer *syncer) cleanCtrlAckBeaconGUIDMap() {
	syncer.ctrlAckBeaconGUIDRWM.Lock()
	defer syncer.ctrlAckBeaconGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ctrlAckBeaconGUID {
		newMap[key] = timestamp
	}
	syncer.ctrlAckBeaconGUID = newMap
}

func (syncer *syncer) cleanBeaconQueryGUIDMap() {
	syncer.beaconQueryGUIDRWM.Lock()
	defer syncer.beaconQueryGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.beaconQueryGUID {
		newMap[key] = timestamp
	}
	syncer.beaconQueryGUID = newMap
}

func (syncer *syncer) cleanCtrlAnswerGUIDMap() {
	syncer.ctrlAnswerGUIDRWM.Lock()
	defer syncer.ctrlAnswerGUIDRWM.Unlock()
	newMap := make(map[string]int64)
	for key, timestamp := range syncer.ctrlAnswerGUID {
		newMap[key] = timestamp
	}
	syncer.ctrlAnswerGUID = newMap
}

func (syncer *syncer) Close() {
	close(syncer.stopSignal)
	syncer.wg.Wait()
	syncer.ctx = nil
}

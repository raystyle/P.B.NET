package controller

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

type syncer struct {
	ctx *CTRL

	expireTime float64

	// key = hex(GUID) value = timestamp
	nodeSendGUID      map[string]int64
	nodeSendGUIDRWM   sync.RWMutex
	nodeAckGUID       map[string]int64
	nodeAckGUIDRWM    sync.RWMutex
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

func newSyncer(ctx *CTRL, config *Config) (*syncer, error) {
	cfg := config.Syncer

	if cfg.ExpireTime < 3*time.Second || cfg.ExpireTime > 30*time.Second {
		return nil, errors.New("expire time < 3 seconds or > 30 seconds")
	}

	syncer := syncer{
		ctx:            ctx,
		expireTime:     cfg.ExpireTime.Seconds(),
		nodeSendGUID:   make(map[string]int64),
		nodeAckGUID:    make(map[string]int64),
		beaconSendGUID: make(map[string]int64),
		beaconAckGUID:  make(map[string]int64),
		queryGUID:      make(map[string]int64),
		stopSignal:     make(chan struct{}),
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
	syncer.cleanNodeSendGUID(now)
	syncer.cleanNodeAckGUID(now)
	syncer.cleanBeaconSendGUID(now)
	syncer.cleanBeaconAckGUID(now)
	syncer.cleanQueryGUID(now)
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
	syncer.cleanNodeSendGUIDMap()
	syncer.cleanNodeAckGUIDMap()
	syncer.cleanBeaconSendGUIDMap()
	syncer.cleanBeaconAckGUIDMap()
	syncer.cleanQueryGUIDMap()
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

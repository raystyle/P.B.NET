package controller

import (
	"sync"
)

type nodeSyncer struct {
	ns *mNodeSyncer
	sync.RWMutex
}

type beaconSyncer struct {
	bs *mBeaconSyncer
	sync.RWMutex
}

type cache struct {
	ctx *CTRL
	// --------------------------------key--------------------------------
	// key = hex(guid)
	nodeKeys      map[string]*mNode
	nodeKeysRWM   sync.RWMutex
	beaconKeys    map[string]*mBeacon
	beaconKeysRWM sync.RWMutex
	// -------------------------------syncer------------------------------
	nodeSyncers        map[string]*nodeSyncer
	nodeSyncersRWM     sync.RWMutex
	beaconSyncers      map[string]*beaconSyncer
	beaconSyncersRWM   sync.RWMutex
	nodeSyncersDB      map[string]*nodeSyncer
	nodeSyncersDBRWM   sync.RWMutex
	beaconSyncersDB    map[string]*beaconSyncer
	beaconSyncersDBRWM sync.RWMutex

	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func newCache(ctx *CTRL) (*cache, error) {
	cache := cache{
		ctx: ctx,
	}
	cache.wg.Add(1)
	go cache.dbSyncer()
	return &cache, nil
}

func (cache *cache) Close() {
	close(cache.stopSignal)
	cache.wg.Wait()
}

// --------------------------------key--------------------------------

func (cache *cache) SelectNodeKey(guid string) *mNode {
	cache.nodeKeysRWM.RLock()
	key := cache.nodeKeys[guid]
	cache.nodeKeysRWM.RUnlock()
	return key
}

func (cache *cache) SelectBeaconKey(guid string) *mBeacon {
	cache.beaconKeysRWM.RLock()
	key := cache.beaconKeys[guid]
	cache.beaconKeysRWM.RUnlock()
	return key
}

func (cache *cache) InsertNodeKey(guid string, key *mNode) {
	cache.nodeKeysRWM.Lock()
	if _, ok := cache.nodeKeys[guid]; !ok {
		cache.nodeKeys[guid] = key
	}
	cache.nodeKeysRWM.Unlock()
}

func (cache *cache) InsertBeaconKey(guid string, key *mBeacon) {
	cache.beaconKeysRWM.Lock()
	if _, ok := cache.beaconKeys[guid]; !ok {
		cache.beaconKeys[guid] = key
	}
	cache.beaconKeysRWM.Unlock()
}

func (cache *cache) DeleteNodeKey(guid string, key *mNode) {
	cache.nodeKeysRWM.Lock()
	delete(cache.nodeKeys, guid)
	cache.nodeKeysRWM.Unlock()
}

func (cache *cache) DeleteBeaconKey(guid string, key *mBeacon) {
	cache.beaconKeysRWM.Lock()
	delete(cache.beaconKeys, guid)
	cache.beaconKeysRWM.Unlock()
}

// --------------------------------sync--------------------------------

func (cache *cache) SelectNodeSyncer(guid string) *mNodeSyncer {
	cache.nodeSyncersRWM.RLock()
	if ns, ok := cache.nodeSyncers[guid]; ok {
		cache.nodeSyncersRWM.RUnlock()
		// maybe changed, must copy
		ns.RLock()
		nsCopy := *ns.ns
		ns.RUnlock()
		return &nsCopy
	} else {
		cache.nodeSyncersRWM.RUnlock()
		return nil
	}
}

func (cache *cache) SelectBeaconSyncer(guid string) *mBeaconSyncer {
	cache.beaconSyncersRWM.RLock()
	if bs, ok := cache.beaconSyncers[guid]; ok {
		cache.beaconSyncersRWM.RUnlock()
		// maybe changed, must copy
		bs.RLock()
		bsCopy := *bs.bs
		bs.RUnlock()
		return &bsCopy
	} else {
		cache.beaconSyncersRWM.RUnlock()
		return nil
	}
}

func (cache *cache) dbSyncer() {
	defer cache.wg.Done()
}

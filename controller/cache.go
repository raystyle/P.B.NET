package controller

import (
	"errors"
	"sync"
)

var (
	ErrCacheNotExist = errors.New("cache doesn't exist")
)

type nodeSyncer struct {
	ns *mNodeSyncer
	sync.RWMutex
}

type beaconSyncer struct {
	bs *mBeaconSyncer
	sync.RWMutex
}

// key = hex(guid)
type cache struct {
	// --------------------------------key--------------------------------
	nodes      map[string]*mNode
	nodesRWM   sync.RWMutex
	beacons    map[string]*mBeacon
	beaconsRWM sync.RWMutex
	// -------------------------------syncer------------------------------
	nodeSyncers      map[string]*nodeSyncer
	nodeSyncersRWM   sync.RWMutex
	beaconSyncers    map[string]*beaconSyncer
	beaconSyncersRWM sync.RWMutex
	// -----------------------------db syncer-----------------------------
	nodeSyncersDB      map[string]*nodeSyncer
	nodeSyncersDBRWM   sync.RWMutex
	beaconSyncersDB    map[string]*beaconSyncer
	beaconSyncersDBRWM sync.RWMutex
}

func newCache() *cache {
	return &cache{
		nodes:           make(map[string]*mNode),
		beacons:         make(map[string]*mBeacon),
		nodeSyncers:     make(map[string]*nodeSyncer),
		beaconSyncers:   make(map[string]*beaconSyncer),
		nodeSyncersDB:   make(map[string]*nodeSyncer),
		beaconSyncersDB: make(map[string]*beaconSyncer),
	}
}

// --------------------------------role--------------------------------

func (cache *cache) SelectNode(guid string) *mNode {
	cache.nodesRWM.RLock()
	key := cache.nodes[guid]
	cache.nodesRWM.RUnlock()
	return key
}

func (cache *cache) SelectBeacon(guid string) *mBeacon {
	cache.beaconsRWM.RLock()
	key := cache.beacons[guid]
	cache.beaconsRWM.RUnlock()
	return key
}

func (cache *cache) InsertNode(guid string, key *mNode) {
	cache.nodesRWM.Lock()
	if _, ok := cache.nodes[guid]; !ok {
		cache.nodes[guid] = key
	}
	cache.nodesRWM.Unlock()
}

func (cache *cache) InsertBeacon(guid string, key *mBeacon) {
	cache.beaconsRWM.Lock()
	if _, ok := cache.beacons[guid]; !ok {
		cache.beacons[guid] = key
	}
	cache.beaconsRWM.Unlock()
}

func (cache *cache) DeleteNode(guid string, key *mNode) {
	cache.nodesRWM.Lock()
	delete(cache.nodes, guid)
	cache.nodesRWM.Unlock()
}

func (cache *cache) DeleteBeacon(guid string, key *mBeacon) {
	cache.beaconsRWM.Lock()
	delete(cache.beacons, guid)
	cache.beaconsRWM.Unlock()
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

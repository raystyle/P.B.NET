package controller

import (
	"encoding/base64"
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

// key = base64(guid)
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

func (cache *cache) SelectNode(guid []byte) *mNode {
	key := base64.StdEncoding.EncodeToString(guid)
	cache.nodesRWM.RLock()
	node := cache.nodes[key]
	cache.nodesRWM.RUnlock()
	return node
}

func (cache *cache) InsertNode(node *mNode) {
	key := base64.StdEncoding.EncodeToString(node.GUID)
	cache.nodesRWM.Lock()
	if _, ok := cache.nodes[key]; !ok {
		cache.nodes[key] = node
	}
	cache.nodesRWM.Unlock()
}

func (cache *cache) DeleteNode(guid string, key *mNode) {
	cache.nodesRWM.Lock()
	delete(cache.nodes, guid)
	cache.nodesRWM.Unlock()
}

func (cache *cache) SelectBeacon(guid []byte) *mBeacon {
	key := base64.StdEncoding.EncodeToString(guid)
	cache.beaconsRWM.RLock()
	beacon := cache.beacons[key]
	cache.beaconsRWM.RUnlock()
	return beacon
}

func (cache *cache) InsertBeacon(beacon *mBeacon) {
	key := base64.StdEncoding.EncodeToString(beacon.GUID)
	cache.beaconsRWM.Lock()
	if _, ok := cache.beacons[key]; !ok {
		cache.beacons[key] = beacon
	}
	cache.beaconsRWM.Unlock()
}

func (cache *cache) DeleteBeacon(guid string, key *mBeacon) {
	cache.beaconsRWM.Lock()
	delete(cache.beacons, guid)
	cache.beaconsRWM.Unlock()
}

// --------------------------------sync--------------------------------

func (cache *cache) SelectNodeSyncer(guid []byte) *mNodeSyncer {
	key := base64.StdEncoding.EncodeToString(guid)
	cache.nodeSyncersRWM.RLock()
	if ns, ok := cache.nodeSyncers[key]; ok {
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

func (cache *cache) InsertNodeSyncer(ns *mNodeSyncer) {
	key := base64.StdEncoding.EncodeToString(ns.GUID)
	cache.nodeSyncersRWM.Lock()
	if _, ok := cache.nodeSyncers[key]; !ok {
		cache.nodeSyncers[key] = &nodeSyncer{ns: ns}
	}
	cache.nodeSyncersRWM.Unlock()
}

func (cache *cache) SelectBeaconSyncer(guid []byte) *mBeaconSyncer {
	key := base64.StdEncoding.EncodeToString(guid)
	cache.beaconSyncersRWM.RLock()
	if bs, ok := cache.beaconSyncers[key]; ok {
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

func (cache *cache) InsertBeaconSyncer(bs *mBeaconSyncer) {
	key := base64.StdEncoding.EncodeToString(bs.GUID)
	cache.beaconSyncersRWM.Lock()
	if _, ok := cache.beaconSyncers[key]; !ok {
		cache.beaconSyncers[key] = &beaconSyncer{bs: bs}
	}
	cache.beaconSyncersRWM.Unlock()
}

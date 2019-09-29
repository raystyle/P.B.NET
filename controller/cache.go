package controller

import (
	"encoding/base64"
	"errors"
	"sync"
)

var (
	errNoCache = errors.New("can't find cache")
)

type nodeSyncer struct {
	*mNodeSyncer
	sync.RWMutex
}

type beaconSyncer struct {
	*mBeaconSyncer
	sync.RWMutex
}

type nodeSyncerDB struct {
	CtrlSend uint64
	NodeRecv uint64
	NodeSend uint64
	CtrlRecv uint64
	sync.RWMutex
}

type beaconSyncerDB struct {
	CtrlSend   uint64
	BeaconRecv uint64
	BeaconSend uint64
	CtrlRecv   uint64
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
	nodeSyncersDB      map[string]*nodeSyncerDB
	nodeSyncersDBRWM   sync.RWMutex
	beaconSyncersDB    map[string]*beaconSyncerDB
	beaconSyncersDBRWM sync.RWMutex
}

func newCache() *cache {
	return &cache{
		nodes:           make(map[string]*mNode),
		beacons:         make(map[string]*mBeacon),
		nodeSyncers:     make(map[string]*nodeSyncer),
		beaconSyncers:   make(map[string]*beaconSyncer),
		nodeSyncersDB:   make(map[string]*nodeSyncerDB),
		beaconSyncersDB: make(map[string]*beaconSyncerDB),
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

// delete nodes nodeSyncers nodeSyncersDB
func (cache *cache) DeleteNode(guid string) {
	cache.nodesRWM.Lock()
	cache.nodeSyncersRWM.Lock()
	cache.nodeSyncersDBRWM.Lock()
	delete(cache.nodes, guid)
	delete(cache.nodeSyncers, guid)
	delete(cache.nodeSyncersDB, guid)
	cache.nodeSyncersDBRWM.Unlock()
	cache.nodeSyncersRWM.Unlock()
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

// delete beacons beaconSyncers beaconSyncersDB
func (cache *cache) DeleteBeacon(guid string) {
	cache.beaconsRWM.Lock()
	cache.beaconSyncersRWM.Lock()
	cache.beaconSyncersDBRWM.Lock()
	delete(cache.beacons, guid)
	delete(cache.beaconSyncers, guid)
	delete(cache.beaconSyncersDB, guid)
	cache.beaconSyncersDBRWM.Unlock()
	cache.beaconSyncersRWM.Unlock()
	cache.beaconsRWM.Unlock()
}

// --------------------------------sync--------------------------------

func (cache *cache) SelectNodeSyncer(guid []byte) *nodeSyncer {
	key := base64.StdEncoding.EncodeToString(guid)
	cache.nodeSyncersRWM.RLock()
	if ns, ok := cache.nodeSyncers[key]; ok {
		cache.nodeSyncersRWM.RUnlock()
		return ns
	} else {
		cache.nodeSyncersRWM.RUnlock()
		return nil
	}
}

func (cache *cache) InsertNodeSyncer(ns *mNodeSyncer) {
	key := base64.StdEncoding.EncodeToString(ns.GUID)
	cache.nodeSyncersRWM.Lock()
	if _, ok := cache.nodeSyncers[key]; !ok {
		cache.nodeSyncers[key] = &nodeSyncer{mNodeSyncer: ns}
		nsDB := nodeSyncerDB{
			CtrlSend: ns.CtrlSend,
			NodeRecv: ns.NodeRecv,
			NodeSend: ns.NodeSend,
			CtrlRecv: ns.CtrlRecv,
		}
		cache.nodeSyncersDBRWM.Lock()
		cache.nodeSyncersDB[key] = &nsDB
		cache.nodeSyncersDBRWM.Unlock()
	}
	cache.nodeSyncersRWM.Unlock()
}

func (cache *cache) SelectBeaconSyncer(guid []byte) *beaconSyncer {
	key := base64.StdEncoding.EncodeToString(guid)
	cache.beaconSyncersRWM.RLock()
	if bs, ok := cache.beaconSyncers[key]; ok {
		cache.beaconSyncersRWM.RUnlock()
		return bs
	} else {
		cache.beaconSyncersRWM.RUnlock()
		return nil
	}
}

func (cache *cache) InsertBeaconSyncer(bs *mBeaconSyncer) {
	key := base64.StdEncoding.EncodeToString(bs.GUID)
	cache.beaconSyncersRWM.Lock()
	if _, ok := cache.beaconSyncers[key]; !ok {
		cache.beaconSyncers[key] = &beaconSyncer{mBeaconSyncer: bs}
		bsDB := beaconSyncerDB{
			CtrlSend:   bs.CtrlSend,
			BeaconRecv: bs.BeaconRecv,
			BeaconSend: bs.BeaconSend,
			CtrlRecv:   bs.CtrlRecv,
		}
		cache.beaconSyncersDBRWM.Lock()
		cache.beaconSyncersDB[key] = &bsDB
		cache.beaconSyncersDBRWM.Unlock()
	}
	cache.beaconSyncersRWM.Unlock()
}

// --------------------------------db sync--------------------------------

func (cache *cache) SelectAllNodeSyncer() map[string]*nodeSyncer {
	cache.nodeSyncersRWM.RLock()
	nsc := make(map[string]*nodeSyncer, len(cache.nodeSyncers))
	for key, ns := range cache.nodeSyncers {
		nsc[key] = ns
	}
	cache.nodeSyncersRWM.RUnlock()
	return nsc
}

func (cache *cache) SelectAllNodeSyncerDB() map[string]*nodeSyncerDB {
	cache.nodeSyncersDBRWM.RLock()
	nsc := make(map[string]*nodeSyncerDB, len(cache.nodeSyncersDB))
	for key, nsDB := range cache.nodeSyncersDB {
		nsc[key] = nsDB
	}
	cache.nodeSyncersDBRWM.RUnlock()
	return nsc
}

func (cache *cache) SelectAllBeaconSyncer() map[string]*beaconSyncer {
	cache.beaconSyncersRWM.RLock()
	bsc := make(map[string]*beaconSyncer, len(cache.beaconSyncers))
	for key, bs := range cache.beaconSyncers {
		bsc[key] = bs
	}
	cache.beaconSyncersRWM.RUnlock()
	return bsc
}

func (cache *cache) SelectAllBeaconSyncerDB() map[string]*beaconSyncerDB {
	cache.beaconSyncersDBRWM.RLock()
	bsc := make(map[string]*beaconSyncerDB, len(cache.beaconSyncersDB))
	for key, bsDB := range cache.beaconSyncersDB {
		bsc[key] = bsDB
	}
	cache.beaconSyncersDBRWM.RUnlock()
	return bsc
}

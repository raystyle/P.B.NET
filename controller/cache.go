package controller

import (
	"sync"

	"project/internal/guid"
)

type cache struct {
	nodes      map[guid.GUID]*mNode
	nodesRWM   sync.RWMutex
	beacons    map[guid.GUID]*mBeacon
	beaconsRWM sync.RWMutex
}

func newCache() *cache {
	return &cache{
		nodes:   make(map[guid.GUID]*mNode),
		beacons: make(map[guid.GUID]*mBeacon),
	}
}

func (cache *cache) SelectNode(guid *guid.GUID) *mNode {
	cache.nodesRWM.RLock()
	defer cache.nodesRWM.RUnlock()
	return cache.nodes[*guid]
}

func (cache *cache) InsertNode(node *mNode) {
	key := guid.GUID{}
	err := key.Write(node.GUID)
	if err != nil {
		panic("cache internal error: " + err.Error())
	}
	cache.nodesRWM.Lock()
	defer cache.nodesRWM.Unlock()
	if _, ok := cache.nodes[key]; !ok {
		cache.nodes[key] = node
	}
}

func (cache *cache) DeleteNode(guid *guid.GUID) {
	cache.nodesRWM.Lock()
	defer cache.nodesRWM.Unlock()
	delete(cache.nodes, *guid)
}

func (cache *cache) SelectBeacon(guid *guid.GUID) *mBeacon {
	cache.beaconsRWM.RLock()
	defer cache.beaconsRWM.RUnlock()
	return cache.beacons[*guid]
}

func (cache *cache) InsertBeacon(beacon *mBeacon) {
	key := guid.GUID{}
	err := key.Write(beacon.GUID)
	if err != nil {
		panic("cache internal error: " + err.Error())
	}
	cache.beaconsRWM.Lock()
	defer cache.beaconsRWM.Unlock()
	if _, ok := cache.beacons[key]; !ok {
		cache.beacons[key] = beacon
	}
}

func (cache *cache) DeleteBeacon(guid *guid.GUID) {
	cache.beaconsRWM.Lock()
	defer cache.beaconsRWM.Unlock()
	delete(cache.beacons, *guid)
}

package node

import (
	"sync"

	"project/internal/guid"
	"project/internal/protocol"
)

type storage struct {
	// key = role GUID
	nodeKeys      map[guid.GUID]*protocol.NodeKey
	nodeKeysRWM   sync.RWMutex
	beaconKeys    map[guid.GUID]*protocol.BeaconKey
	beaconKeysRWM sync.RWMutex
}

func newStorage() *storage {
	storage := storage{
		nodeKeys:   make(map[guid.GUID]*protocol.NodeKey),
		beaconKeys: make(map[guid.GUID]*protocol.BeaconKey),
	}
	return &storage
}

func (storage *storage) GetNodeKey(guid *guid.GUID) *protocol.NodeKey {
	storage.nodeKeysRWM.RLock()
	defer storage.nodeKeysRWM.RUnlock()
	return storage.nodeKeys[*guid]
}

func (storage *storage) AddNodeKey(guid *guid.GUID, sk *protocol.NodeKey) {
	storage.nodeKeysRWM.Lock()
	defer storage.nodeKeysRWM.Unlock()
	if _, ok := storage.nodeKeys[*guid]; !ok {
		storage.nodeKeys[*guid] = sk
	}
}

func (storage *storage) DeleteNodeKey(guid *guid.GUID) {
	storage.nodeKeysRWM.Lock()
	defer storage.nodeKeysRWM.Unlock()
	delete(storage.nodeKeys, *guid)
}

func (storage *storage) GetAllNodeKeys() map[guid.GUID]*protocol.NodeKey {
	nodeKeys := make(map[guid.GUID]*protocol.NodeKey)
	storage.nodeKeysRWM.RLock()
	defer storage.nodeKeysRWM.RUnlock()
	for key, value := range storage.nodeKeys {
		nodeKeys[key] = value
	}
	return nodeKeys
}

func (storage *storage) GetBeaconKey(guid *guid.GUID) *protocol.BeaconKey {
	storage.beaconKeysRWM.RLock()
	defer storage.beaconKeysRWM.RUnlock()
	return storage.beaconKeys[*guid]
}

func (storage *storage) AddBeaconKey(guid *guid.GUID, sk *protocol.BeaconKey) {
	storage.beaconKeysRWM.Lock()
	defer storage.beaconKeysRWM.Unlock()
	if _, ok := storage.beaconKeys[*guid]; !ok {
		storage.beaconKeys[*guid] = sk
	}
}

func (storage *storage) DeleteBeaconKey(guid *guid.GUID) {
	storage.beaconKeysRWM.Lock()
	defer storage.beaconKeysRWM.Unlock()
	delete(storage.beaconKeys, *guid)
}

func (storage *storage) GetAllBeaconKeys() map[guid.GUID]*protocol.BeaconKey {
	beaconKeys := make(map[guid.GUID]*protocol.BeaconKey)
	storage.beaconKeysRWM.RLock()
	defer storage.beaconKeysRWM.RUnlock()
	for key, value := range storage.beaconKeys {
		beaconKeys[key] = value
	}
	return beaconKeys
}

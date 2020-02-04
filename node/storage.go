package node

import (
	"sync"
	"time"

	"project/internal/guid"
	"project/internal/messages"
)

type storage struct {
	// key = role GUID
	nodeRegisters    map[guid.GUID]chan *messages.NodeRegisterResponse
	nodeRegistersM   sync.Mutex
	beaconRegisters  map[guid.GUID]chan *messages.BeaconRegisterResponse
	beaconRegistersM sync.Mutex

	// key = role GUID
	nodeSessionKeys      map[guid.GUID]*nodeSessionKey
	nodeSessionKeysRWM   sync.RWMutex
	beaconSessionKeys    map[guid.GUID]*beaconSessionKey
	beaconSessionKeysRWM sync.RWMutex
}

type nodeSessionKey struct {
	PublicKey    []byte
	KexPublicKey []byte
	AckTime      time.Time
}

type beaconSessionKey struct {
	PublicKey    []byte
	KexPublicKey []byte
	AckTime      time.Time
}

func newStorage() *storage {
	storage := storage{
		nodeRegisters:     make(map[guid.GUID]chan *messages.NodeRegisterResponse),
		beaconRegisters:   make(map[guid.GUID]chan *messages.BeaconRegisterResponse),
		nodeSessionKeys:   make(map[guid.GUID]*nodeSessionKey),
		beaconSessionKeys: make(map[guid.GUID]*beaconSessionKey),
	}
	return &storage
}

func (storage *storage) CreateNodeRegister(guid *guid.GUID) <-chan *messages.NodeRegisterResponse {
	storage.nodeRegistersM.Lock()
	defer storage.nodeRegistersM.Unlock()
	if _, ok := storage.nodeRegisters[*guid]; !ok {
		c := make(chan *messages.NodeRegisterResponse, 1)
		storage.nodeRegisters[*guid] = c
		return c
	}
	return nil
}

func (storage *storage) SetNodeRegister(guid *guid.GUID, response *messages.NodeRegisterResponse) {
	storage.nodeRegistersM.Lock()
	defer storage.nodeRegistersM.Unlock()
	if nr, ok := storage.nodeRegisters[*guid]; ok {
		nr <- response
		close(nr)
		delete(storage.nodeRegisters, *guid)
	}
}

func (storage *storage) GetNodeSessionKey(guid *guid.GUID) *nodeSessionKey {
	storage.nodeSessionKeysRWM.RLock()
	defer storage.nodeSessionKeysRWM.RUnlock()
	return storage.nodeSessionKeys[*guid]
}

func (storage *storage) AddNodeSessionKey(guid *guid.GUID, sk *nodeSessionKey) {
	storage.nodeSessionKeysRWM.Lock()
	defer storage.nodeSessionKeysRWM.Unlock()
	if _, ok := storage.nodeSessionKeys[*guid]; !ok {
		storage.nodeSessionKeys[*guid] = sk
	}
}

func (storage *storage) DeleteNodeSessionKey(guid *guid.GUID) {
	storage.nodeSessionKeysRWM.Lock()
	defer storage.nodeSessionKeysRWM.Unlock()
	delete(storage.nodeSessionKeys, *guid)
}

func (storage *storage) CreateBeaconRegister(guid *guid.GUID) <-chan *messages.BeaconRegisterResponse {
	storage.beaconRegistersM.Lock()
	defer storage.beaconRegistersM.Unlock()
	if _, ok := storage.beaconRegisters[*guid]; !ok {
		c := make(chan *messages.BeaconRegisterResponse, 1)
		storage.beaconRegisters[*guid] = c
		return c
	}
	return nil
}

func (storage *storage) SetBeaconRegister(guid *guid.GUID, response *messages.BeaconRegisterResponse) {
	storage.beaconRegistersM.Lock()
	defer storage.beaconRegistersM.Unlock()
	if nr, ok := storage.beaconRegisters[*guid]; ok {
		nr <- response
		close(nr)
		delete(storage.beaconRegisters, *guid)
	}
}

func (storage *storage) GetBeaconSessionKey(guid *guid.GUID) *beaconSessionKey {
	storage.beaconSessionKeysRWM.RLock()
	defer storage.beaconSessionKeysRWM.RUnlock()
	return storage.beaconSessionKeys[*guid]
}

func (storage *storage) AddBeaconSessionKey(guid *guid.GUID, sk *beaconSessionKey) {
	storage.beaconSessionKeysRWM.Lock()
	defer storage.beaconSessionKeysRWM.Unlock()
	if _, ok := storage.beaconSessionKeys[*guid]; !ok {
		storage.beaconSessionKeys[*guid] = sk
	}
}

func (storage *storage) DeleteBeaconSessionKey(guid *guid.GUID) {
	storage.beaconSessionKeysRWM.Lock()
	defer storage.beaconSessionKeysRWM.Unlock()
	delete(storage.beaconSessionKeys, *guid)
}

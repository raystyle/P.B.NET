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
	nodeKeys      map[guid.GUID]*nodeKey
	nodeKeysRWM   sync.RWMutex
	beaconKeys    map[guid.GUID]*beaconKey
	beaconKeysRWM sync.RWMutex
}

func newStorage() *storage {
	storage := storage{
		nodeRegisters:   make(map[guid.GUID]chan *messages.NodeRegisterResponse),
		beaconRegisters: make(map[guid.GUID]chan *messages.BeaconRegisterResponse),
		nodeKeys:        make(map[guid.GUID]*nodeKey),
		beaconKeys:      make(map[guid.GUID]*beaconKey),
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

type nodeKey struct {
	PublicKey    []byte
	KexPublicKey []byte
	ReplyTime    time.Time
}

type beaconKey struct {
	PublicKey    []byte
	KexPublicKey []byte
	ReplyTime    time.Time
}

func (storage *storage) GetNodeKey(guid *guid.GUID) *nodeKey {
	storage.nodeKeysRWM.RLock()
	defer storage.nodeKeysRWM.RUnlock()
	return storage.nodeKeys[*guid]
}

func (storage *storage) AddNodeKey(guid *guid.GUID, sk *nodeKey) {
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

func (storage *storage) GetBeaconKey(guid *guid.GUID) *beaconKey {
	storage.beaconKeysRWM.RLock()
	defer storage.beaconKeysRWM.RUnlock()
	return storage.beaconKeys[*guid]
}

func (storage *storage) AddBeaconKey(guid *guid.GUID, sk *beaconKey) {
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

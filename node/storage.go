package node

import (
	"encoding/hex"
	"sync"
	"time"

	"project/internal/guid"
)

type storage struct {
	// key = hex(role guid) lower
	nodeRegisters    map[string]chan uint8 // register result
	nodeRegistersM   sync.Mutex
	beaconRegisters  map[string]chan uint8 // register result
	beaconRegistersM sync.Mutex

	// key = hex(role guid) lower
	nodeSessionKeys      map[string]*nodeSessionKey
	nodeSessionKeysRWM   sync.RWMutex
	beaconSessionKeys    map[string]*beaconSessionKey
	beaconSessionKeysRWM sync.RWMutex

	hexPool sync.Pool
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
		nodeRegisters:     make(map[string]chan uint8),
		beaconRegisters:   make(map[string]chan uint8),
		nodeSessionKeys:   make(map[string]*nodeSessionKey),
		beaconSessionKeys: make(map[string]*beaconSessionKey),
	}
	storage.hexPool.New = func() interface{} {
		return make([]byte, 2*guid.Size)
	}
	return &storage
}

func (storage *storage) calculateKey(guid []byte) string {
	dst := storage.hexPool.Get().([]byte)
	defer storage.hexPool.Put(dst)
	hex.Encode(dst, guid)
	return string(dst)
}

func (storage *storage) CreateNodeRegister(guid []byte) <-chan uint8 {
	key := storage.calculateKey(guid)
	storage.nodeRegistersM.Lock()
	defer storage.nodeRegistersM.Unlock()
	if _, ok := storage.nodeRegisters[key]; !ok {
		c := make(chan uint8, 1)
		storage.nodeRegisters[key] = c
		return c
	}
	return nil
}

func (storage *storage) CreateBeaconRegister(guid []byte) <-chan uint8 {
	key := storage.calculateKey(guid)
	storage.beaconRegistersM.Lock()
	defer storage.beaconRegistersM.Unlock()
	if _, ok := storage.beaconRegisters[key]; !ok {
		c := make(chan uint8, 1)
		storage.beaconRegisters[key] = c
		return c
	}
	return nil
}

func (storage *storage) SetNodeRegister(guid []byte, result uint8) {
	key := storage.calculateKey(guid)
	storage.nodeRegistersM.Lock()
	defer storage.nodeRegistersM.Unlock()
	if nr, ok := storage.nodeRegisters[key]; ok {
		nr <- result
		close(nr)
		delete(storage.nodeRegisters, key)
	}
}

func (storage *storage) SetBeaconRegister(guid []byte, result uint8) {
	key := storage.calculateKey(guid)
	storage.beaconRegistersM.Lock()
	defer storage.beaconRegistersM.Unlock()
	if nr, ok := storage.beaconRegisters[key]; ok {
		nr <- result
		close(nr)
		delete(storage.beaconRegisters, key)
	}
}

func (storage *storage) GetNodeSessionKey(guid []byte) *nodeSessionKey {
	key := storage.calculateKey(guid)
	storage.nodeSessionKeysRWM.RLock()
	defer storage.nodeSessionKeysRWM.RUnlock()
	return storage.nodeSessionKeys[key]
}

func (storage *storage) GetBeaconSessionKey(guid []byte) *beaconSessionKey {
	key := storage.calculateKey(guid)
	storage.beaconSessionKeysRWM.RLock()
	defer storage.beaconSessionKeysRWM.RUnlock()
	return storage.beaconSessionKeys[key]
}

func (storage *storage) AddNodeSessionKey(guid []byte, sk *nodeSessionKey) {
	key := storage.calculateKey(guid)
	storage.nodeSessionKeysRWM.RLock()
	defer storage.nodeSessionKeysRWM.RUnlock()
	if _, ok := storage.nodeSessionKeys[key]; !ok {
		storage.nodeSessionKeys[key] = sk
	}
}

func (storage *storage) AddBeaconSessionKey(guid []byte, sk *beaconSessionKey) {
	key := storage.calculateKey(guid)
	storage.beaconSessionKeysRWM.RLock()
	defer storage.beaconSessionKeysRWM.RUnlock()
	if _, ok := storage.beaconSessionKeys[key]; !ok {
		storage.beaconSessionKeys[key] = sk
	}
}

func (storage *storage) DeleteNodeSessionKey(guid []byte) {
	key := storage.calculateKey(guid)
	storage.nodeSessionKeysRWM.RLock()
	defer storage.nodeSessionKeysRWM.RUnlock()
	delete(storage.nodeSessionKeys, key)
}

func (storage *storage) DeleteBeaconSessionKey(guid []byte) {
	key := storage.calculateKey(guid)
	storage.beaconSessionKeysRWM.RLock()
	defer storage.beaconSessionKeysRWM.RUnlock()
	delete(storage.beaconSessionKeys, key)
}

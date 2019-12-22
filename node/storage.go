package node

import (
	"sync"
)

type storage struct {
	// role register
	nodeRegisters      map[string]string
	nodeRegistersRWM   sync.RWMutex
	beaconRegisters    map[string]string
	beaconRegistersRWM sync.RWMutex

	// role public key
	nodePublicKeys       map[string]*nodeSessionKey
	nodePublicKeysRWM    sync.RWMutex
	beaconSessionKeys    map[string]*beaconSessionKey
	beaconSessionKeysRWM sync.RWMutex
}

func newStorage() *storage {
	return &storage{}
}

type nodeSessionKey struct {
}

type beaconSessionKey struct {
}

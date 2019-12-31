package node

import (
	"sync"

	"project/internal/messages"
)

// bootMgr is used to get bootstrap nodes
type bootMgr struct {
	ctx *Node

	// key = messages.Bootstrap.Tag
	bootstraps    map[string]*messages.Bootstrap
	bootstrapsRWM sync.RWMutex
}

func newBootManager(ctx *Node, config *Config) *bootMgr {
	mgr := bootMgr{
		ctx:        ctx,
		bootstraps: make(map[string]*messages.Bootstrap),
	}
	return &mgr
}

func (bm *bootMgr) Close() {
	bm.ctx = nil
}

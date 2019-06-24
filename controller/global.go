package controller

import (
	"sync"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
)

type global struct {
	ctx        *CONTROLLER
	proxy      *proxyclient.PROXY
	dns        *dnsclient.DNS
	timesync   *timesync.TIMESYNC
	object     map[uint32]interface{}
	object_rwm sync.RWMutex
	conf_err   error
	conf_once  sync.Once
	wg         sync.WaitGroup
}

func new_global(ctx *CONTROLLER) (*global, error) {
	g := &global{
		ctx: ctx,
	}
	return g, nil
}

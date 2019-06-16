package node

import (
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
)

type global_config struct {
}

type global struct {
	proxy    *proxyclient.PROXY
	dns      *dnsclient.DNS
	timesync *timesync.TIMESYNC
}

func new_global(c *global_config) (*global, error) {

}

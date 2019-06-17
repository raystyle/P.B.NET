package node

import (
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
)

type global struct {
	proxy    *proxyclient.PROXY
	dns      *dnsclient.DNS
	timesync *timesync.TIMESYNC
}

func new_global(c *Config) (*global, error) {
	proxy, err := proxyclient.New()

	dns, err := dnsclient.New()

	timesync, err := timesync.New()

}

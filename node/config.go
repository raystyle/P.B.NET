package node

import (
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
	"project/internal/logger"
)

type Config struct {
	// log
	Log_level logger.Level `toml:"log_level"`
	// global
	Proxy_Clients    map[string]*proxyclient.Client `toml:"proxy_clients"`
	DNS_Clients      map[string]*dnsclient.Client   `toml:"dns_clients"`
	TimeSync_Clients map[string]*timesync.Client    `toml:"timesync_clients"`
}

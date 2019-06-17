package node

import (
	"time"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
)

type Config struct {
	// log
	Log_level string `toml:"log_level"`
	// global
	Proxy_Clients      map[string]*proxyclient.Client `toml:"proxy_clients"`
	DNS_Clients        map[string]*dnsclient.Client   `toml:"dns_clients"`
	DNS_Cache_Deadline time.Duration                  `toml:"dns_cache_deadline"`
	Timesync_Clients   map[string]*timesync.Client    `toml:"timesync_clients"`
	Timesync_Interval  time.Duration                  `toml:"timesync_interval"`
}

const object_key_max = 1048575

// runtime env
// 0 < key < 1048576
type object_key = int

const (
	// external object
	ctrl_ecdsa object_key = iota // verify controller
	ctrl_rsa                     // encrypt session key
	ctrl_aes                     // decrypt controller broadcast message

	// internal object
	node_guid         // identification
	node_guid_encrypt // update self sync_send_height
	database_aes      // encrypt self data
	startup_time      // first bootstrap time
	certificate       // for listener
	session_ecdsa
	session_key

	// sync_send
	sync_send_height

	// confuse object
	confusion_00
	confusion_01
	confusion_02
	confusion_03
	confusion_04
	confusion_05
	confusion_06
)

package node

import (
	"time"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
	"project/internal/logger"
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

// before create a node need check config
func (this *Config) Check() error {
	node := &NODE{config: this}
	_, err := new_logger(node)
	if err != nil {
		return err
	}
	node.logger = logger.Discard
	g, err := new_global(node)
	if err != nil {
		return err
	}
	node.global = g
	err = node.global.timesync.Test()
	if err != nil {
		return err
	}
	return nil
}

const object_key_max uint32 = 1048575

// runtime env
// 0 < key < 1048576
type object_key = uint32

const (
	// external object
	ctrl_ed25519 object_key = iota // verify controller
	ctrl_aes                       // decrypt controller broadcast message

	// internal object
	node_guid           // identification
	node_guid_encrypted // update self sync_send_height
	database_aes        // encrypt self data
	startup_time        // first bootstrap time
	certificate         // for listener
	session_ed25519
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

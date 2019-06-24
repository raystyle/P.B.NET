package node

import (
	"time"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
	"project/internal/logger"
	"project/internal/messages"
)

type Config struct {
	Log_level string

	// global
	Proxy_Clients      map[string]*proxyclient.Client
	DNS_Clients        map[string]*dnsclient.Client
	DNS_Cache_Deadline time.Duration
	Timesync_Clients   map[string]*timesync.Client
	Timesync_Interval  time.Duration
	// controller
	CTRL_ED25519_PUB []byte
	CTRL_AES_Key     []byte // key + iv
	// register only resolve success once
	Is_Genesis          bool   // use controller to register
	Register_AES_Key    []byte //  key + iv Config is encrypted
	Register_Bootstraps []*messages.Bootstrap

	// server
	Conn_Limit int
	Listeners  []*messages.Listener
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

func (this *Config) Build() {

}

const object_key_max uint32 = 1048575

// runtime env
// 0 < key < 1048576
type object_key = uint32

const (
	// external object
	// verify controller role & message
	ctrl_ed25519_pub object_key = iota
	// decrypt controller broadcast message
	ctrl_aes_key

	// internal object
	node_guid     // identification
	node_guid_enc // update self sync_send_height
	database_aes  // encrypt self data
	startup_time  // global.configure time
	certificate   // for listener
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

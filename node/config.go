package node

import (
	"time"

	"project/internal/config"
	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/timesync"
)

type Debug struct {
	SkipTimeSyncer bool
}

type Config struct {
	Debug Debug

	// CheckMode is used to check whether
	// the configuration is correct
	CheckMode bool

	// logger
	LogLevel string

	// global
	ProxyClients       map[string]*proxy.Client
	DNSServers         map[string]*dns.Server
	DnsCacheDeadline   time.Duration
	TimeSyncerConfigs  map[string]*timesync.Config
	TimeSyncerInterval time.Duration

	// controller
	CtrlED25519 []byte // public key
	CtrlAESKey  []byte // key + iv

	// register
	IsGenesis          bool   // use controller to register
	RegisterAESKey     []byte // key + iv Config is encrypted
	RegisterBootstraps []*config.Bootstrap

	// server
	ConnLimit        int
	HandshakeTimeout time.Duration
	Listeners        []*config.Listener
}

// before create a node need check config
func (cfg *Config) Check() error {
	cfg.CheckMode = true
	node, err := New(cfg)
	if err != nil {
		return err
	}
	err = node.global.timeSyncer.Test()
	if err != nil {
		return err
	}
	return nil
}

func (cfg *Config) Build() {

}

// runtime env
// 0 < key < 1048576
const objectKeyMax uint32 = 1048575

type objectKey = uint32

const (
	// external object
	ctrlED25519   objectKey = iota // verify controller role & message
	ctrlAESCrypto                  // decrypt controller broadcast message

	// internal object
	nodeGUID    // identification
	nodeGUIDEnc // update self syncSendHeight
	dbAESCrypto // encrypt self data(database)
	startupTime // global.configure time
	certificate // for listener
	sessionED25519
	sessionKey

	// sync message
	syncSendHeight // sync send

	// confuse object
	confusion00
	confusion01
	confusion02
	confusion03
	confusion04
	confusion05
	confusion06
)

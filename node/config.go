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
	// TODO skip encode
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

	// controller configs
	CtrlPublicKey   []byte // ed25519
	CtrlExPublicKey []byte // curve25519
	CtrlAESCrypto   []byte // key + iv

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
	err = node.global.dnsClient.Test()
	if err != nil {
		return err
	}
	err = node.global.timeSyncer.Test()
	if err != nil {
		return err
	}
	node.global.Destroy()
	return nil
}

func (cfg *Config) Build() {

}

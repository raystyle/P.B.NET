package node

import (
	"time"

	"github.com/vmihailenco/msgpack/v4"

	"project/internal/dns"
	"project/internal/messages"
	"project/internal/proxy"
	"project/internal/timesync"
)

type Debug struct {
	SkipTimeSyncer bool

	// from controller
	Broadcast chan []byte
	Send      chan []byte
}

type Config struct {
	Debug Debug `toml:"-"`

	// CheckMode is used to check whether
	// the configuration is correct
	CheckMode bool `toml:"-"`

	LogLevel string `toml:"log_level"`

	Global struct {
		DNSCacheExpire   time.Duration `toml:"dns_cache_expire"`
		TimeSyncInterval time.Duration `toml:"time_sync_interval"`

		ProxyClients      map[string]*proxy.Chain     `toml:"-"`
		DNSServers        map[string]*dns.Server      `toml:"-"`
		TimeSyncerConfigs map[string]*timesync.Config `toml:"-"`
	} `toml:"global"`

	Sender struct {
		MaxBufferSize int `toml:"max_buffer_size"`
		Worker        int `toml:"worker"`
		QueueSize     int `toml:"queue_size"`
		MaxConns      int `toml:"max_conn"`
	} `toml:"sender"`

	Syncer struct {
		MaxBufferSize int           `toml:"max_buffer_size"`
		Worker        int           `toml:"worker"`
		QueueSize     int           `toml:"queue_size"`
		ExpireTime    time.Duration `toml:"expire_time"`
	} `toml:"syncer"`

	Register struct {
		Bootstraps []*messages.Bootstrap `toml:"-"`
	} `toml:"register"`

	Server struct {
		MaxConns int `toml:"max_conns"` // single listener
		// key = tag
		Listeners []*messages.Listener `toml:"-"`
	} `toml:"server"`

	// controller configs
	CTRL struct {
		PublicKey   []byte // ed25519
		ExPublicKey []byte // curve25519
		AESCrypto   []byte // key + iv
	} `toml:"-"`
}

// before create a node need check config
func (cfg *Config) Check() error {
	cfg.CheckMode = true
	node, err := New(cfg)
	if err != nil {
		return err
	}
	defer node.Exit(nil)
	err = node.global.dnsClient.TestServers()
	if err != nil {
		return err
	}
	err = node.global.timeSyncer.Test()
	if err != nil {
		return err
	}
	return nil
}

func (cfg *Config) Build() ([]byte, error) {
	return msgpack.Marshal(cfg)
}

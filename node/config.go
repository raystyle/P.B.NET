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

	// test sender
	BroadcastChan chan []byte
}

type Config struct {
	// TODO skip encode
	Debug Debug `toml:"-"`

	// CheckMode is used to check whether
	// the configuration is correct
	CheckMode bool `toml:"-"`

	// logger
	LogLevel string `toml:"log_level"`

	// database
	DBFilePath string `toml:"db_file_path"`
	DBUsername string `toml:"db_username"`
	DBPassword string `toml:"db_password"`

	// global
	ProxyClients       map[string]*proxy.Client    `toml:"proxy_clients"`
	DNSServers         map[string]*dns.Server      `toml:"dns_servers"`
	DnsCacheDeadline   time.Duration               `toml:"dns_cache_deadline"`
	TimeSyncerConfigs  map[string]*timesync.Config `toml:"time_syncer_configs"`
	TimeSyncerInterval time.Duration               `toml:"time_syncer_interval"`

	// sender
	MaxBufferSize   int `toml:"max_buffer_size"` // syncer also use it
	SenderWorker    int `toml:"sender_worker"`
	SenderQueueSize int `toml:"sender_queue_size"`

	// syncer
	MaxSyncerClient  int           `toml:"max_syncer_client"`
	SyncerWorker     int           `toml:"syncer_worker"`
	SyncerQueueSize  int           `toml:"syncer_queue_size"`
	ReserveWorker    int           `toml:"reserve_worker"`
	RetryTimes       int           `toml:"retry_times"`
	RetryInterval    time.Duration `toml:"retry_interval"`
	BroadcastTimeout time.Duration `toml:"broadcast_timeout"`

	// controller configs
	CtrlPublicKey   []byte `toml:"-"` // ed25519
	CtrlExPublicKey []byte `toml:"-"` // curve25519
	CtrlAESCrypto   []byte `toml:"-"` // key + iv

	// register
	IsGenesis          bool                `toml:"-"` // use controller to register
	RegisterAESKey     []byte              `toml:"-"` // key + iv Config is encrypted
	RegisterBootstraps []*config.Bootstrap `toml:"-"`

	// server
	ConnLimit        int                `toml:"conn_limit"`
	HandshakeTimeout time.Duration      `toml:"handshake_timeout"`
	Listeners        []*config.Listener `toml:"listeners"`
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

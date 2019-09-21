package beacon

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
	CtrlPublicKey   []byte `toml:"-"`
	CtrlExPublicKey []byte `toml:"-"`

	// register
	RegisterAESKey     []byte              `toml:"-"` // key + iv Config is encrypted
	RegisterBootstraps []*config.Bootstrap `toml:"-"`
}

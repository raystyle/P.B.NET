package controller

import (
	"time"
)

type Debug struct {
	SkipTimeSyncer bool

	// see handler.go
	NodeSend   chan []byte // Node send test message
	BeaconSend chan []byte // Beacon send test message
}

type Config struct {
	Debug Debug `toml:"-"`

	// logger
	LogLevel string `toml:"log_level"`

	// database
	Dialect         string        `toml:"dialect"` // "mysql"
	DSN             string        `toml:"dsn"`
	DBLogFile       string        `toml:"db_log_file"`
	DBMaxOpenConns  int           `toml:"db_max_open_conns"`
	DBMaxIdleConns  int           `toml:"db_max_idle_conns"`
	GORMLogFile     string        `toml:"gorm_log_file"`
	GORMDetailedLog bool          `toml:"gorm_detailed_log"`
	DBSyncInterval  time.Duration `toml:"db_sync_interval"` // cache

	// global
	BuiltinDir         string        `toml:"builtin_dir"`
	KeyDir             string        `toml:"key_dir"`
	DNSCacheDeadline   time.Duration `toml:"dns_cache_deadline"`
	TimeSyncerInterval time.Duration `toml:"time_syncer_interval"`

	// sender
	MaxBufferSize   int `toml:"max_buffer_size"` // syncer also use it
	SenderWorker    int `toml:"sender_worker"`
	SenderQueueSize int `toml:"sender_queue_size"`

	// syncer
	MaxSyncerClient int `toml:"max_syncer_client"`
	SyncerWorker    int `toml:"syncer_worker"`
	SyncerQueueSize int `toml:"syncer_queue_size"`
	MessageTimeout  int `toml:"message_timeout"` // TODO rename

	// web server
	HTTPSAddress  string `toml:"https_address"`
	HTTPSCertFile string `toml:"https_cert_file"`
	HTTPSKeyFile  string `toml:"https_key_file"`
	HTTPSWebDir   string `toml:"https_web_dir"`
	HTTPSUsername string `toml:"https_username"`
	HTTPSPassword string `toml:"https_password"`
}

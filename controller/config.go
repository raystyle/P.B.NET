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

	LogLevel string `toml:"log_level"`

	Database struct {
		Dialect         string `toml:"dialect"` // "mysql"
		DSN             string `toml:"dsn"`
		MaxOpenConns    int    `toml:"max_open_conns"`
		MaxIdleConns    int    `toml:"max_idle_conns"`
		LogFile         string `toml:"log_file"`
		GORMLogFile     string `toml:"gorm_log_file"`
		GORMDetailedLog bool   `toml:"gorm_detailed_log"`
	} `toml:"database"`

	Global struct {
		BuiltinDir       string        `toml:"builtin_dir"`
		KeyDir           string        `toml:"key_dir"`
		DNSCacheExpire   time.Duration `toml:"dns_cache_expire"`
		TimeSyncInterval time.Duration `toml:"time_sync_interval"`
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

	Web struct {
		Dir      string `toml:"dir"`
		CertFile string `toml:"cert_file"`
		KeyFile  string `toml:"key_file"`
		Address  string `toml:"address"`
		Username string `toml:"username"` // super user
		Password string `toml:"password"`
	} `toml:"web"`
}

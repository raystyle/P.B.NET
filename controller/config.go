package controller

import (
	"io"
	"time"

	"project/internal/dns"
)

// Config include configuration about Controller
type Config struct {
	Debug Debug `toml:"-"`

	Database struct {
		Dialect         string `toml:"dialect"` // "mysql"
		DSN             string `toml:"dsn"`
		MaxOpenConns    int    `toml:"max_open_conns"`
		MaxIdleConns    int    `toml:"max_idle_conns"`
		LogFile         string `toml:"log_file"`
		GORMLogFile     string `toml:"gorm_log_file"`
		GORMDetailedLog bool   `toml:"gorm_detailed_log"`
	} `toml:"database"`

	Logger struct {
		Level  string    `toml:"level"`
		File   string    `toml:"file"`
		Writer io.Writer `toml:"-"`
	} `toml:"logger"`

	Global struct {
		DNSCacheExpire   time.Duration `toml:"dns_cache_expire"`
		TimeSyncInterval time.Duration `toml:"time_sync_interval"`
	} `toml:"global"`

	// TODO client Options
	Client struct { // options
		ProxyTag string        `toml:"proxy_tag"`
		Timeout  time.Duration `toml:"timeout"`
		DNSOpts  dns.Options   `toml:"dns"`
	} `toml:"client"`

	Sender struct {
		MaxConns      int           `toml:"max_conn"`
		Worker        int           `toml:"worker"`
		Timeout       time.Duration `toml:"timeout"`
		QueueSize     int           `toml:"queue_size"`
		MaxBufferSize int           `toml:"max_buffer_size"`
	} `toml:"sender"`

	Syncer struct {
		ExpireTime time.Duration `toml:"expire_time"`
	} `toml:"syncer"`

	Worker struct {
		Number        int `toml:"number"`
		QueueSize     int `toml:"queue_size"`
		MaxBufferSize int `toml:"max_buffer_size"`
	} `toml:"worker"`

	Web struct {
		Dir      string `toml:"dir"`
		CertFile string `toml:"cert_file"`
		KeyFile  string `toml:"key_file"`
		Address  string `toml:"address"`
		Username string `toml:"username"` // super user
		Password string `toml:"password"`
	} `toml:"web"`
}

// Debug is used to test
type Debug struct {
	SkipSynchronizeTime bool

	// see handler.go
	NodeSend   chan []byte // Node send test message
	BeaconSend chan []byte // Beacon send test message
}

// copy Config.Client
type opts struct {
	ProxyTag string
	Timeout  time.Duration
	DNSOpts  dns.Options
}

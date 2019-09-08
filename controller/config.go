package controller

import (
	"time"
)

type debug struct {
	skipTimeSyncer bool
}

type Config struct {
	debug debug

	// logger
	LogLevel string `toml:"log_level"`

	// global
	BuiltinDir         string        `toml:"builtin_dir"`
	KeyDir             string        `toml:"key_dir"`
	DNSCacheDeadline   time.Duration `toml:"dns_cache_deadline"`
	TimeSyncerInterval time.Duration `toml:"time_syncer_interval"`

	// database
	Dialect         string `toml:"dialect"` // "mysql"
	DSN             string `toml:"dsn"`
	DBLogFile       string `toml:"db_log_file"`
	DBMaxOpenConns  int    `toml:"db_max_open_conns"`
	DBMaxIdleConns  int    `toml:"db_max_idle_conns"`
	GORMLogFile     string `toml:"gorm_log_file"`
	GORMDetailedLog bool   `toml:"gorm_detailed_log"`

	// web server
	HTTPSAddress  string `toml:"https_address"`
	HTTPSCertFile string `toml:"https_cert_file"`
	HTTPSKeyFile  string `toml:"https_key_file"`
	WebDir        string `toml:"web_dir"`
}

type objectKey = uint32

const (
	// verify controller role & sign message
	ed25519PrivateKey objectKey = iota
	ed25519PublicKey
	curve25519PublicKey // for key exchange
	// encrypt controller broadcast message
	aesCrypto
)

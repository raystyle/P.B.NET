package controller

import (
	"time"
)

type Config struct {
	// debug
	bin_path string

	// logger
	Log_Level string `toml:"log_level"`

	// global
	DNS_Cache_Deadline time.Duration `toml:"dns_cache_deadline"`
	Timesync_Interval  time.Duration `toml:"timesync_interval"`

	// database
	Dialect           string `toml:"dialect"` // "mysql"
	DSN               string `toml:"dsn"`
	DB_Log_Path       string `toml:"db_log_path"`
	DB_Max_Open_Conns int    `toml:"db_max_open_conns"`
	DB_Max_Idle_Conns int    `toml:"db_max_idle_conns"`
	GORM_Log_Path     string `toml:"gorm_log_path"`
	GORM_Detailed_Log bool   `toml:"gorm_detailed_log"`
}

const object_key_max uint32 = 1048575

type object_key = uint32

const (
	// external object
	// verify controller role & message
	ed25519_privatekey object_key = iota
	ed25519_publickey
	// encrypt controller broadcast message
	aes_cryptor
)

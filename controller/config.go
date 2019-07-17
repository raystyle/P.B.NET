package controller

import (
	"time"
)

type debug struct {
	skip_timesync bool
}

type Config struct {
	debug debug

	// logger
	Log_Level string `toml:"log_level"`

	// global
	Builtin_Dir        string        `toml:"builtin_dir"`
	Key_Dir            string        `toml:"key_dir"`
	DNS_Cache_Deadline time.Duration `toml:"dns_cache_deadline"`
	Timesync_Interval  time.Duration `toml:"timesync_interval"`

	// database
	Dialect           string `toml:"dialect"` // "mysql"
	DSN               string `toml:"dsn"`
	DB_Log_File       string `toml:"db_log_file"`
	DB_Max_Open_Conns int    `toml:"db_max_open_conns"`
	DB_Max_Idle_Conns int    `toml:"db_max_idle_conns"`
	GORM_Log_File     string `toml:"gorm_log_file"`
	GORM_Detailed_Log bool   `toml:"gorm_detailed_log"`

	// web server
	HTTPS_Address   string `toml:"https_address"`
	HTTPS_Cert_File string `toml:"https_cert_file"`
	HTTPS_Key_File  string `toml:"https_key_file"`
	Web_Dir         string `toml:"web_dir"`
}

type object_key = uint32

const (
	// verify controller role & sign message
	ed25519_privatekey object_key = iota
	ed25519_publickey
	curve25519_publickey // for key exchange
	// encrypt controller broadcast message
	aes_cryptor
)

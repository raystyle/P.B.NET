package controller

import (
	"time"
)

func testGenerateConfig() *Config {
	c := Config{
		Debug: Debug{
			SkipTimeSyncer: true,
		},

		LogLevel:        "debug",
		MaxSyncerClient: 8,

		Database: struct {
			Dialect         string `toml:"dialect"`
			DSN             string `toml:"dsn"`
			MaxOpenConns    int    `toml:"max_open_conns"`
			MaxIdleConns    int    `toml:"max_idle_conns"`
			LogFile         string `toml:"log_file"`
			GORMLogFile     string `toml:"gorm_log_file"`
			GORMDetailedLog bool   `toml:"gorm_detailed_log"`
		}{
			Dialect:         "mysql",
			DSN:             "root:123456@tcp(127.0.0.1:3306)/p.b.net_test?loc=Local&parseTime=true",
			MaxOpenConns:    16,
			MaxIdleConns:    16,
			LogFile:         "../app/log/database.log",
			GORMLogFile:     "../app/log/gorm.log",
			GORMDetailedLog: false,
		},

		Global: struct {
			BuiltinDir         string        `toml:"builtin_dir"`
			KeyDir             string        `toml:"key_dir"`
			DNSCacheDeadline   time.Duration `toml:"dns_cache_deadline"`
			TimeSyncerInterval time.Duration `toml:"time_syncer_interval"`
		}{
			BuiltinDir:         "../app/builtin",
			KeyDir:             "../app/key",
			DNSCacheDeadline:   3 * time.Minute,
			TimeSyncerInterval: 5 * time.Minute,
		},

		Sender: struct {
			MaxBufferSize int `toml:"max_buffer_size"`
			Worker        int `toml:"worker"`
			QueueSize     int `toml:"queue_size"`
		}{
			MaxBufferSize: 16384,
			Worker:        64,
			QueueSize:     512,
		},

		Syncer: struct {
			MaxBufferSize int           `toml:"max_buffer_size"`
			Worker        int           `toml:"worker"`
			QueueSize     int           `toml:"queue_size"`
			Timeout       time.Duration `toml:"timeout"`
		}{

			MaxBufferSize: 16384,
			Worker:        64,
			QueueSize:     512,
			Timeout:       2 * time.Minute,
		},

		Web: struct {
			Dir      string `toml:"dir"`
			CertFile string `toml:"cert_file"`
			KeyFile  string `toml:"key_file"`
			Address  string `toml:"address"`
			Username string `toml:"username"`
			Password string `toml:"password"`
		}{
			Address:  "localhost:9931",
			CertFile: "../app/cert/server.crt",
			KeyFile:  "../app/cert/server.key",
			Dir:      "../app/web",
			Username: "admin",
			Password: "56c10b0f6a18abe0247c31fd1d1a70e51e5a09f2", // admin159**
		},
	}
	return &c
}

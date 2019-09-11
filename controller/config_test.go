package controller

import (
	"runtime"
	"time"
)

func testGenerateConfig() *Config {
	const (
		dialect = "mysql"
		dsn     = "root:123456@tcp(127.0.0.1:3306)/p.b.net_test?loc=Local&parseTime=true"
	)
	c := &Config{
		// logger
		LogLevel: "debug",

		// global
		DNSCacheDeadline:   3 * time.Minute,
		TimeSyncerInterval: 15 * time.Minute,
		BuiltinDir:         "../app/builtin",
		KeyDir:             "../app/key",

		// database
		Dialect:         dialect,
		DSN:             dsn,
		DBMaxOpenConns:  16,
		DBMaxIdleConns:  16,
		DBLogFile:       "../app/log/database.log",
		GORMLogFile:     "../app/log/gorm.log",
		GORMDetailedLog: false,

		// sender
		BufferSize:      4096,
		SenderNumber:    runtime.NumCPU(),
		SenderQueueSize: 512,

		// syncer
		MaxSyncer:        2,
		WorkerNumber:     64,
		WorkerQueueSize:  512,
		ReserveWorker:    16,
		RetryTimes:       3,
		RetryInterval:    5 * time.Second,
		BroadcastTimeout: 30 * time.Second,
		ReceiveTimeout:   30 * time.Second,
		DBSyncInterval:   time.Second,

		// web server
		HTTPSAddress:  "localhost:9931",
		HTTPSCertFile: "../app/cert/server.crt",
		HTTPSKeyFile:  "../app/cert/server.key",
		HTTPSWebDir:   "../app/web",
		HTTPSUsername: "admin",
		HTTPSPassword: "56c10b0f6a18abe0247c31fd1d1a70e51e5a09f2",
	}
	c.Debug.SkipTimeSyncer = true
	return c
}

package controller

import (
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

		// http server
		HTTPSAddress:  "localhost:9931",
		HTTPSCertFile: "../app/cert/server.crt",
		HTTPSKeyFile:  "../app/cert/server.key",
		WebDir:        "../app/web",
	}
	c.Debug.SkipTimeSyncer = true
	return c
}

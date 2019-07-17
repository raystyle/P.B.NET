package controller

import (
	"time"
)

func test_gen_config() *Config {
	const (
		test_dialect = "mysql"
		test_dsn     = "root:asH*dg122@tcp(127.0.0.1:3306)/p.b.net_test?loc=Local&parseTime=true"
	)
	c := &Config{
		// logger
		Log_Level: "debug",

		// global
		DNS_Cache_Deadline: 3 * time.Minute,
		Timesync_Interval:  15 * time.Minute,
		Builtin_Dir:        "../app/builtin",
		Key_Dir:            "../app/key",

		// database
		Dialect:           test_dialect,
		DSN:               test_dsn,
		DB_Max_Open_Conns: 16,
		DB_Max_Idle_Conns: 16,
		DB_Log_File:       "../app/log/database.log",
		GORM_Log_File:     "../app/log/gorm.log",
		GORM_Detailed_Log: false,

		// http server
		HTTPS_Address:   ":9931",
		HTTPS_Cert_File: "../app/cert/server.crt",
		HTTPS_Key_File:  "../app/cert/server.key",
		Web_Dir:         "../app/web",
	}
	c.debug.skip_timesync = true
	return c
}

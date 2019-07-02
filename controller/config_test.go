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
		Key_Path:           "../app/key/ctrl.key",
		// database
		Dialect:           test_dialect,
		DSN:               test_dsn,
		DB_Max_Open_Conns: 16,
		DB_Max_Idle_Conns: 16,
		DB_Log_Path:       "../app/log/database.log",
		GORM_Log_Path:     "../app/log/gorm.log",
		GORM_Detailed_Log: false,
		// http server
		HTTP_Address: ":9931",
	}
	return c
}

package controller

import (
	"time"
)

const (
	test_dialect = "mysql"
	test_dsn     = "root:asH*dg122@tcp(127.0.0.1:3306)/p.b.net_test?loc=Local&parseTime=true"
)

func test_gen_config() *Config {
	c := &Config{
		// debug
		bin_path: "../app",
		// logger
		Log_Level: "debug",
		// global
		DNS_Cache_Deadline: 3 * time.Minute,
		Timesync_Interval:  15 * time.Minute,
		// database
		Dialect:           test_dialect,
		DSN:               test_dsn,
		DB_Max_Open_Conns: 16,
		DB_Max_Idle_Conns: 16,
		DB_Log_Path:       "log/database.log",
		GORM_Log_Path:     "log/gorm.log",
		GORM_Detailed_Log: false,
	}
	return c
}

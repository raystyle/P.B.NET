package controller

import (
	"time"
)

type Config struct {
	Log_Level string

	// global
	DNS_Cache_Deadline time.Duration
	Timesync_Interval  time.Duration

	// database
	Database          string // "mysql" dialect
	DSN               string // config
	Database_Log      string
	Gorm_Log          string
	DB_Max_Open_Conns int
	DB_Max_Idle_Conn  int
}

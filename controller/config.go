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
	Dialect           string // "mysql"
	DSN               string
	DB_Log_Path       string
	DB_Max_Open_Conns int
	DB_Max_Idle_Conn  int
	GORM_Log_Path     string
	GORM_Detailed_Log bool
}

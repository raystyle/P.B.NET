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
	Database string // "mysql" dialect
	DSN      string // config
}

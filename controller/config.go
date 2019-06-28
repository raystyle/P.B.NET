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

const object_key_max uint32 = 1048575

type object_key = uint32

const (
	// external object
	// verify controller role & message
	ed25519_privatekey object_key = iota
	ed25519_publickey
	// encrypt controller broadcast message
	aes_cryptor
)

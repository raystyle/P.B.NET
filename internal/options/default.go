package options

import "time"

const (
	// general default options
	DefaultMaxConns         = 1000
	DefaultDeadline         = 30 * time.Second
	DefaultDialTimeout      = 30 * time.Second
	DefaultHandshakeTimeout = 30 * time.Second

	// role global default options
	DefaultDNSCacheExpireTime = 3 * time.Minute
	DefaultTimeSyncFixed      = 10 // second
	DefaultTimeSyncRandom     = 20 // second
	DefaultTimeSyncInterval   = 10 * time.Minute
)

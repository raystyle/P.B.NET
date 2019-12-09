package options

import "time"

const (
	// general default options
	DefaultMaxConns         = 1000
	DefaultDeadline         = 30 * time.Second
	DefaultDialTimeout      = 30 * time.Second
	DefaultHandshakeTimeout = 30 * time.Second

	// role global default options
	DefaultCacheExpireTime = 3 * time.Minute  // dns client
	DefaultSyncInterval    = 10 * time.Minute // timesyncer
)

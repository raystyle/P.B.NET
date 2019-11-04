package options

import "time"

const (
	DefaultMaxConns         = 1000
	DefaultDeadline         = 30 * time.Second
	DefaultDialTimeout      = 30 * time.Second
	DefaultHandshakeTimeout = 30 * time.Second

	DefaultCacheExpireTime = 3 * time.Minute  // dns client
	DefaultSyncInterval    = 10 * time.Minute // timesyncer
)

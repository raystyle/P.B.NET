package options

import "time"

const (
	// role.global
	DefaultCacheExpireTime = 2 * time.Minute  // dns client
	DefaultSyncInterval    = 10 * time.Minute // timesyncer

	// xnet
	DefaultMaxConns         = 1000
	DefaultDeadline         = time.Minute
	DefaultDialTimeout      = time.Minute
	DefaultHandshakeTimeout = time.Minute
)

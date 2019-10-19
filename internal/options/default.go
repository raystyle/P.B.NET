package options

import "time"

const (
	// role.global
	DefaultCacheExpireTime = 2 * time.Minute
	DefaultSyncInterval    = 15 * time.Minute // timesyncer

	// xnet
	DefaultStartTimeout     = 250 * time.Millisecond // max serve timeout
	DefaultConnectionLimit  = 1000                   // server max connection
	DefaultDialTimeout      = time.Minute
	DefaultHandshakeTimeout = time.Minute
)

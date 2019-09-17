package options

import "time"

const (
	DefaultStartTimeout     = 250 * time.Millisecond // max serve timeout
	DefaultConnectionLimit  = 10000                  // server max connection
	DefaultDialTimeout      = time.Minute
	DefaultHandshakeTimeout = time.Minute
)

package options

import "time"

const (
	DefaultStartTimeout     = 250 * time.Millisecond // max serve timeout
	DefaultConnectionLimit  = 10000                  // server max connection
	DefaultDialTimeout      = 30 * time.Second
	DefaultHandshakeTimeout = 15 * time.Second
)

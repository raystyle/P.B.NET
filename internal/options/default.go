package options

import "time"

const (
	DEFAULT_START_TIMEOUT     = 250 * time.Millisecond
	DEFAULT_DIAL_TIMEOUT      = 30 * time.Second
	DEFAULT_HANDSHAKE_TIMEOUT = 15 * time.Second
	DEFAULT_CONNECTION_LIMIT  = 10000
)

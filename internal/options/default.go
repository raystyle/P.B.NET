package options

import "time"

const (
	DEFAULT_START_TIMEOUT     = 250 * time.Millisecond
	DEFAULT_DIAL_TIMEOUT      = time.Minute
	DEFAULT_HANDSHAKE_TIMEOUT = time.Minute
	DEFAULT_CONNECTION_LIMIT  = 10000
)

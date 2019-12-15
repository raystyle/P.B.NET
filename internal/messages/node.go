package messages

import (
	"time"

	"project/internal/options"
)

// Listener is used to listen a listener to a Node
type Listener struct {
	Tag       string
	Mode      string
	Network   string
	Address   string
	Timeout   time.Duration
	TLSConfig options.TLSConfig
}

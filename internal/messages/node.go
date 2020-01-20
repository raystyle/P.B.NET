package messages

import (
	"time"

	"project/internal/option"
)

// Listener is used to listen a listener to a Node
type Listener struct {
	Tag       string
	Mode      string
	Network   string
	Address   string
	Timeout   time.Duration
	TLSConfig option.TLSConfig
}

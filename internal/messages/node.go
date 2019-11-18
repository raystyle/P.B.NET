package messages

import (
	"time"

	"project/internal/options"
)

type Listener struct {
	Tag       string
	Mode      string
	Network   string
	Address   string
	Timeout   time.Duration
	TLSConfig options.TLSConfig
}

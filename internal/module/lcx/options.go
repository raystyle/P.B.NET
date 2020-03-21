package lcx

import (
	"time"
)

const (
	defaultDialTimeout    = 15 * time.Second
	defaultConnectTimeout = 10 * time.Second
	defaultMaxConnections = 1000
)

// Options contains all options about Tranner, Slaver and Listener.
type Options struct {
	LocalNetwork string
	LocalAddress string
	MaxConns     int
	Timeout      time.Duration
}

func (opts *Options) apply() *Options {
	nOpts := *opts
	if nOpts.LocalNetwork == "" {
		nOpts.LocalNetwork = "tcp"
	}
	if nOpts.LocalAddress == "" {
		nOpts.LocalAddress = ":0"
	}
	if nOpts.MaxConns < 1 {
		nOpts.MaxConns = defaultMaxConnections
	}
	if nOpts.Timeout < 1 {
		nOpts.Timeout = defaultConnectTimeout
	}
	return &nOpts
}

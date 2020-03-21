package lcx

import (
	"time"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultDialTimeout    = 15 * time.Second
	defaultMaxConnections = 1000
)

// Options contains all options about Tranner, Slaver and Listener.
type Options struct {
	// tran and listener
	LocalNetwork string
	LocalAddress string

	// tran and slave, connect target timeout
	ConnectTimeout time.Duration

	// only slave, connect listener timeout
	DialTimeout time.Duration

	// tran, slave and listener
	MaxConns int
}

func (opts *Options) apply() *Options {
	nOpts := *opts
	if nOpts.LocalNetwork == "" {
		nOpts.LocalNetwork = "tcp"
	}
	if nOpts.LocalAddress == "" {
		nOpts.LocalAddress = ":0"
	}
	if nOpts.ConnectTimeout < 1 {
		nOpts.ConnectTimeout = defaultConnectTimeout
	}
	if nOpts.DialTimeout < 1 {
		nOpts.DialTimeout = defaultDialTimeout
	}
	if nOpts.MaxConns < 1 {
		nOpts.MaxConns = defaultMaxConnections
	}
	return &nOpts
}

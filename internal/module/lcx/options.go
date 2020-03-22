package lcx

import (
	"time"
)

// about default options
const (
	DefaultConnectTimeout = 10 * time.Second
	DefaultDialTimeout    = 15 * time.Second
	DefaultMaxConnections = 1000
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
		nOpts.ConnectTimeout = DefaultConnectTimeout
	}
	if nOpts.DialTimeout < 1 {
		nOpts.DialTimeout = DefaultDialTimeout
	}
	if nOpts.MaxConns < 1 {
		nOpts.MaxConns = DefaultMaxConnections
	}
	return &nOpts
}

package lcx

import (
	"time"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultMaxConnections = 1000
	defaultDialTimeout    = 15 * time.Second
)

// Options contains all options about Tranner, Slaver and Listener.
type Options struct {
	LocalNetwork string
	LocalAddress string

	// tran and slave
	ConnectTimeout time.Duration
	MaxConns       int

	// only slaver
	DialTimeout time.Duration
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
	if nOpts.ConnectTimeout < 1 {
		nOpts.ConnectTimeout = defaultConnectTimeout
	}
	if nOpts.DialTimeout < 1 {
		nOpts.DialTimeout = defaultDialTimeout
	}
	return &nOpts
}

// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx

import (
	"net"
	"sync"
)

// from golang.org/x/net/netutil/listen.go

// LimitListener returns a Listener that accepts at most n simultaneous
// connections from the provided Listener.
func Limit_Listener(l net.Listener, n int) net.Listener {
	return &limit_listener{
		Listener: l,
		sem:      make(chan struct{}, n),
		done:     make(chan struct{}),
	}
}

type limit_listener struct {
	net.Listener
	sem       chan struct{}
	closeOnce sync.Once     // ensures the done chan is only closed once
	done      chan struct{} // no values sent; closed when Close is called
}

// acquire acquires the limiting semaphore. Returns true if successfully
// accquired, false if the listener is closed and the semaphore is not
// acquired.
func (l *limit_listener) acquire() bool {
	select {
	case <-l.done:
		return false
	case l.sem <- struct{}{}:
		return true
	}
}
func (l *limit_listener) release() { <-l.sem }

func (l *limit_listener) Accept() (net.Conn, error) {
	acquired := l.acquire()
	// If the semaphore isn't acquired because the listener was closed, expect
	// that this call to accept won't block, but immediately return an error.
	c, err := l.Listener.Accept()
	if err != nil {
		if acquired {
			l.release()
		}
		return nil, err
	}
	return &limit_listener_conn{Conn: c, release: l.release}, nil
}

func (l *limit_listener) Close() error {
	err := l.Listener.Close()
	l.closeOnce.Do(func() { close(l.done) })
	return err
}

type limit_listener_conn struct {
	net.Conn
	releaseOnce sync.Once
	release     func()
}

func (l *limit_listener_conn) Close() error {
	err := l.Conn.Close()
	l.releaseOnce.Do(l.release)
	return err
}

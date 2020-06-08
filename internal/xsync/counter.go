package xsync

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Counter is used to wait all resource closed like connection in Server program.
type Counter struct {
	count int32
}

// Add is used to add delta, it will panic if count is negative.
func (c *Counter) Add(delta int32) {
	count := atomic.AddInt32(&c.count, delta)
	if count < 0 {
		const format = "xsync: negative counter %d in Add()"
		panic(fmt.Sprintf(format, count))
	}
}

// Done decrements the counter by one.
func (c *Counter) Done() {
	c.Add(-1)
}

// Wait is used to wait the count be zero, it will panic if count is negative.
func (c *Counter) Wait() {
	addr := &c.count
	const maxDelay = time.Second
	var delay time.Duration
	for {
		count := atomic.LoadInt32(addr)
		switch {
		case count == 0:
			return
		case count < 0:
			const format = "xsync: negative counter %d in Wait()"
			panic(fmt.Sprintf(format, count))
		}
		// wait until count equal zero
		if delay == 0 {
			delay = 5 * time.Millisecond
		} else {
			delay *= 2
		}
		if delay > maxDelay {
			delay = maxDelay
		}
		time.Sleep(delay)
	}
}

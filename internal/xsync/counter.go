package xsync

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Counter is used to wait all resource closed like connection and goroutine
// in Server program. It also can use like sync.WaitGroup.
type Counter struct {
	count int64
}

// Add is used to add delta, it will panic if count is negative.
func (c *Counter) Add(delta int) {
	count := atomic.AddInt64(&c.count, int64(delta))
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
	const maxDelay = time.Second
	var (
		count int64
		delay time.Duration
	)
	addr := &c.count
	for {
		count = atomic.LoadInt64(addr)
		switch {
		case count == 0:
			return
		case count < 0:
			const format = "xsync: negative counter %d in Wait()"
			panic(fmt.Sprintf(format, count))
		}
		// wait loop until count equal zero
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

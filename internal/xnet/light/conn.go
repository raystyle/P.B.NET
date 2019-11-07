package light

import (
	"context"
	"net"
	"sync"
	"time"

	"project/internal/options"
)

type Conn struct {
	ctx context.Context
	net.Conn
	isClient         bool
	handshakeTimeout time.Duration
	handshakeErr     error
	handshakeM       sync.Mutex
	handshakeOnce    sync.Once
	crypto           *crypto
}

func (c *Conn) Handshake() error {
	c.handshakeM.Lock()
	defer c.handshakeM.Unlock()
	if c.handshakeErr != nil {
		return c.handshakeErr
	}
	c.handshakeOnce.Do(func() {
		// default handshake timeout
		if c.handshakeTimeout < 1 {
			c.handshakeTimeout = options.DefaultHandshakeTimeout
		}
		// context
		done := make(chan struct{})
		defer close(done)
		go func() {
			defer func() { recover() }()
			select {
			case <-done:
			case <-c.ctx.Done():
				_ = c.Conn.Close()
			}
		}()
		c.handshakeErr = c.SetDeadline(time.Now().Add(c.handshakeTimeout))
		if c.handshakeErr != nil {
			return
		}
		if c.isClient {
			c.handshakeErr = c.clientHandshake()
		} else {
			c.handshakeErr = c.serverHandshake()
		}
		if c.handshakeErr != nil {
			return
		}
		c.handshakeErr = c.SetDeadline(time.Time{})
		if c.handshakeErr != nil {
			return
		}
	})
	return c.handshakeErr
}

func (c *Conn) Read(b []byte) (n int, err error) {
	err = c.Handshake()
	if err != nil {
		return
	}
	n, err = c.Conn.Read(b)
	if err != nil {
		return
	}
	c.crypto.decrypt(b)
	return n, nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	err = c.Handshake()
	if err != nil {
		return
	}
	return c.Conn.Write(c.crypto.encrypt(b))
}

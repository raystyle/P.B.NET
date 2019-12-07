package light

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

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
		if c.handshakeTimeout < 1 {
			c.handshakeTimeout = options.DefaultHandshakeTimeout
		}
		// interrupt
		errChan := make(chan error, 1)
		wg := sync.WaitGroup{}
		done := make(chan struct{})
		defer func() {
			close(done)
			wg.Wait()
		}()
		wg.Add(1)
		go func() {
			defer func() {
				recover()
				wg.Done()
			}()
			select {
			case <-done:
			case <-time.After(c.handshakeTimeout):
				errChan <- errors.New("handshake timeout")
				_ = c.Conn.Close()
			case <-c.ctx.Done():
				errChan <- c.ctx.Err()
				_ = c.Conn.Close()
			}
		}()
		if c.isClient {
			c.handshakeErr = c.clientHandshake()
		} else {
			c.handshakeErr = c.serverHandshake()
		}
		select {
		case err := <-errChan:
			c.handshakeErr = err
		default:
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

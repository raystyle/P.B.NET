package light

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const defaultHandshakeTimeout = 30 * time.Second

// Conn implement net.Conn
type Conn struct {
	ctx context.Context
	net.Conn
	isClient         bool
	handshakeTimeout time.Duration
	handshakeErr     error
	handshakeOnce    sync.Once
	crypto           *crypto
	closeOnce        sync.Once
}

// Handshake is used to handshake with client or server
func (c *Conn) Handshake() error {
	c.handshakeOnce.Do(func() {
		if c.handshakeTimeout < 1 {
			c.handshakeTimeout = defaultHandshakeTimeout
		}
		// interrupt
		wg := sync.WaitGroup{}
		errChan := make(chan error, 1)
		done := make(chan struct{})
		wg.Add(1)
		go func() {
			defer func() {
				recover()
				close(errChan)
				wg.Done()
			}()
			timer := time.NewTimer(c.handshakeTimeout)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
				errChan <- errors.New("handshake timeout")
				_ = c.Close()
			case <-c.ctx.Done():
				errChan <- c.ctx.Err()
				_ = c.Close()
			}
		}()
		defer func() {
			close(done)
			wg.Wait()
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

// Read reads data from the connection
func (c *Conn) Read(b []byte) (n int, err error) {
	err = c.Handshake()
	if err != nil {
		return
	}
	n, err = c.Conn.Read(b)
	if err != nil {
		return
	}
	c.crypto.Decrypt(b)
	return n, nil
}

// Write writes data to the connection
func (c *Conn) Write(b []byte) (n int, err error) {
	err = c.Handshake()
	if err != nil {
		return
	}
	return c.Conn.Write(c.crypto.Encrypt(b))
}

// Close is used to close the connection
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.Conn.Close()
	})
	return err
}

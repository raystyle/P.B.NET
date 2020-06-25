package light

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/xpanic"
)

const defaultHandshakeTimeout = 30 * time.Second

// Conn implement net.Conn.
type Conn struct {
	net.Conn

	ctx context.Context

	isClient bool

	handshakeTimeout time.Duration
	handshakeErr     error
	handshakeOnce    sync.Once

	crypto    *crypto
	closeOnce sync.Once
}

// Handshake is used to handshake with client or server.
func (c *Conn) Handshake() error {
	c.handshakeOnce.Do(func() {
		c.handshakeErr = c.handshake()
	})
	return c.handshakeErr
}

func (c *Conn) handshake() error {
	if c.handshakeTimeout < 1 {
		c.handshakeTimeout = defaultHandshakeTimeout
	}
	_ = c.Conn.SetDeadline(time.Now().Add(c.handshakeTimeout))
	// interrupt
	var errCh chan error
	if c.ctx.Done() != nil {
		errCh = make(chan error, 2)
	}
	var err error
	if errCh == nil {
		if c.isClient {
			err = c.clientHandshake()
		} else {
			err = c.serverHandshake()
		}
	} else {
		go func() {
			defer close(errCh)
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Log(r, "Conn.Handshake")
					errCh <- errors.New(buf.String())
				}
			}()
			if c.isClient {
				errCh <- c.clientHandshake()
			} else {
				errCh <- c.serverHandshake()
			}
		}()
		select {
		case err = <-errCh:
			if err != nil {
				// if the error was due to the context
				// closing, prefer the context's error, rather
				// than some random network teardown error.
				if e := c.ctx.Err(); e != nil {
					err = e
				}
			}
		case <-c.ctx.Done():
			err = c.ctx.Err()
		}
	}
	if err != nil {
		_ = c.Close()
		return err
	}
	return c.Conn.SetDeadline(time.Time{})
}

// Read reads data from the connection.
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

// Write writes data to the connection.
func (c *Conn) Write(b []byte) (n int, err error) {
	err = c.Handshake()
	if err != nil {
		return
	}
	return c.Conn.Write(c.crypto.Encrypt(b))
}

// Close is used to close the connection.
func (c *Conn) Close() (err error) {
	c.closeOnce.Do(func() {
		err = c.Conn.Close()
	})
	return
}

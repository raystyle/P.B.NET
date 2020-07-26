package xtls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"

	"project/internal/xpanic"
)

const defaultDialTimeout = 30 * time.Second

// Server is a link.
func Server(conn net.Conn, cfg *tls.Config) *tls.Conn {
	return tls.Server(conn, cfg)
}

// Client is a link.
func Client(conn net.Conn, cfg *tls.Config) *tls.Conn {
	return tls.Client(conn, cfg)
}

// Listen is a link.
func Listen(network, address string, config *tls.Config) (net.Listener, error) {
	return tls.Listen(network, address, config)
}

// Dial is used to dial a connection with context.Background().
func Dial(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*tls.Conn, error) {
	return DialContext(context.Background(), network, address, config, timeout, dialContext)
}

// DialContext is used to dial a connection with context.
// If dialContext is nil, dialContext = new(net.Dialer).DialContext.
func DialContext(
	ctx context.Context,
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*tls.Conn, error) {
	// set server name
	if config.ServerName == "" {
		colonPos := strings.LastIndex(address, ":")
		if colonPos == -1 {
			return nil, errors.New("missing port in address")
		}
		hostname := address[:colonPos]
		c := config.Clone()
		c.ServerName = hostname
		config = c
	}
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	if dialContext == nil {
		dialContext = new(net.Dialer).DialContext
	}
	// dial raw connection.
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rawConn, err := dialContext(dialCtx, network, address)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(rawConn, config)
	_ = tlsConn.SetDeadline(time.Now().Add(timeout))
	// interrupt
	var errCh chan error
	if ctx.Done() != nil {
		errCh = make(chan error, 2)
	}
	if errCh == nil {
		err = tlsConn.Handshake()
	} else {
		go func() {
			defer close(errCh)
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Log(r, "DialContext")
					errCh <- fmt.Errorf(buf.String())
				}
			}()
			errCh <- tlsConn.Handshake()
		}()
		select {
		case err = <-errCh:
			if err != nil {
				// if the error was due to the context
				// closing, prefer the context's error, rather
				// than some random network teardown error.
				if e := ctx.Err(); e != nil {
					err = e
				}
			}
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	if err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	_ = tlsConn.SetDeadline(time.Time{})
	return tlsConn, nil
}

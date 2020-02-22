package xtls

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/xpanic"
)

const defaultDialTimeout = 30 * time.Second

// Server is a link
func Server(conn net.Conn, cfg *tls.Config) *tls.Conn {
	return tls.Server(conn, cfg)
}

// Client is a link
func Client(conn net.Conn, cfg *tls.Config) *tls.Conn {
	return tls.Client(conn, cfg)
}

// Listen is a link
func Listen(network, address string, config *tls.Config) (net.Listener, error) {
	return tls.Listen(network, address, config)
}

// Dial is used to dial a connection with context.Background()
func Dial(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*tls.Conn, error) {
	return DialContext(context.Background(), network, address, config, timeout, dialContext)
}

// DialContext is used to dial a connection with context
// if dialContext is nil, dialContext = new(net.Dialer).DialContext
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
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rawConn, err := dialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(rawConn, config)

	// interrupt
	wg := sync.WaitGroup{}
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(xpanic.Print(r, "DialContext"))
			}
			wg.Done()
		}()
		select {
		case <-done:
		case <-ctx.Done():
			_ = tlsConn.Close()
		}
	}()
	defer func() {
		close(done)
		wg.Wait()
	}()

	err = tlsConn.Handshake()
	if err != nil {
		return nil, err
	}
	return tlsConn, nil
}

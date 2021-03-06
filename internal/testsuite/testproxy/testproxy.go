package testproxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/cert"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/testsuite/testcert"
)

// server tags
const (
	TagSocks5  = "p1"
	TagHTTP    = "p2"
	TagBalance = "balance"
)

// PoolAndManager is used to create a proxy pool with balance and proxy manager.
func PoolAndManager(t *testing.T) (*proxy.Pool, *proxy.Manager, *cert.Pool) {
	certPool := testcert.CertPool(t)

	// create proxy manager
	manager := proxy.NewManager(certPool, logger.Test, nil)

	// add socks5 server
	err := manager.Add(&proxy.Server{
		Tag:  TagSocks5,
		Mode: proxy.ModeSocks5,
	})
	require.NoError(t, err)

	// add http proxy server
	err = manager.Add(&proxy.Server{
		Tag:  TagHTTP,
		Mode: proxy.ModeHTTP,
	})
	require.NoError(t, err)

	// start all proxy servers
	for _, server := range manager.Servers() {
		go func(server *proxy.Server) {
			err := server.ListenAndServe("tcp", "localhost:0")
			require.NoError(t, err)
		}(server)
	}
	time.Sleep(250 * time.Millisecond)

	// create proxy pool
	proxyPool := proxy.NewPool(certPool)

	// add socks5 client
	server, err := manager.Get(TagSocks5)
	require.NoError(t, err)
	err = proxyPool.Add(&proxy.Client{
		Tag:     TagSocks5,
		Mode:    proxy.ModeSocks5,
		Network: "tcp",
		Address: server.Addresses()[0].String(),
	})
	require.NoError(t, err)

	// add http proxy client
	server, err = manager.Get(TagHTTP)
	require.NoError(t, err)
	err = proxyPool.Add(&proxy.Client{
		Tag:     TagHTTP,
		Mode:    proxy.ModeHTTP,
		Network: "tcp",
		Address: server.Addresses()[0].String(),
	})
	require.NoError(t, err)

	// add balance
	err = proxyPool.Add(&proxy.Client{
		Tag:     TagBalance,
		Mode:    proxy.ModeBalance,
		Options: `tags = ["p1", "p2"]`,
	})
	require.NoError(t, err)
	return proxyPool, manager, certPool
}

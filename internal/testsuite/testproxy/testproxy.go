package testproxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/proxy"
)

const (
	tagSocks5 = "test proxy 1"
	tagHTTP   = "test proxy 2"

	TagBalance = "balance"
)

// PoolAndManager is used to create a proxy pool
// with balance and proxy manager
func PoolAndManager(t *testing.T) (*proxy.Pool, *proxy.Manager) {
	// create proxy server manager
	manager := proxy.NewManager(logger.Test, nil)
	// add socks5 server
	err := manager.Add(&proxy.Server{
		Tag:  tagSocks5,
		Mode: proxy.ModeSocks,
	})
	require.NoError(t, err)
	// add http proxy server
	err = manager.Add(&proxy.Server{
		Tag:  tagHTTP,
		Mode: proxy.ModeHTTP,
	})
	require.NoError(t, err)
	// start all proxy servers
	for _, server := range manager.Servers() {
		require.NoError(t, server.ListenAndServe("tcp", "localhost:0"))
	}

	// create proxy client pool
	pool := proxy.NewPool()
	// add socks5 client
	server, err := manager.Get(tagSocks5)
	require.NoError(t, err)
	err = pool.Add(&proxy.Client{
		Tag:     tagSocks5,
		Mode:    proxy.ModeSocks,
		Network: "tcp",
		Address: server.Address(),
	})
	require.NoError(t, err)
	// add http proxy client
	server, err = manager.Get(tagHTTP)
	require.NoError(t, err)
	err = pool.Add(&proxy.Client{
		Tag:     tagHTTP,
		Mode:    proxy.ModeHTTP,
		Network: "tcp",
		Address: server.Address(),
	})
	require.NoError(t, err)

	// add balance
	err = pool.Add(&proxy.Client{
		Tag:     TagBalance,
		Mode:    proxy.ModeBalance,
		Options: `tags = ["test proxy 1","test proxy 2"]`,
	})
	require.NoError(t, err)
	return pool, manager
}

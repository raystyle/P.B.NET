package testproxy

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestProxyPoolAndManager(t *testing.T) {
	manager, pool := ProxyPoolAndManager(t)
	defer func() {
		require.NoError(t, manager.Close())
		testsuite.IsDestroyed(t, manager)
	}()

	// test balance
	balance, err := pool.Get(TagBalance)
	require.NoError(t, err)

	// create http client
	transport := new(http.Transport)
	balance.HTTP(transport)
	client := http.Client{
		Transport: transport,
		Timeout:   time.Minute,
	}
	defer client.CloseIdleConnections()
	resp, err := client.Get(testsuite.GetHTTPS())
	require.NoError(t, err)
	testsuite.HTTPResponse(t, resp)

	testsuite.IsDestroyed(t, pool)
}

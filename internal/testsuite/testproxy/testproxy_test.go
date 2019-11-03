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

	// test http client
	transport := new(http.Transport)
	balance.HTTP(transport)
	client := http.Client{
		Transport: transport,
		Timeout:   time.Minute,
	}
	defer client.CloseIdleConnections()
	testsuite.HTTPClient(t, &client, testsuite.GetHTTP())

	testsuite.IsDestroyed(t, pool)
}

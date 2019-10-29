package testproxy

import (
	"io/ioutil"
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
	transport := http.Transport{DialContext: balance.DialContext}
	client := http.Client{
		Transport: &transport,
		Timeout:   time.Minute,
	}
	defer client.CloseIdleConnections()
	resp, err := client.Get("http://www.msftconnecttest.com/connecttest.txt")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer func() { _ = resp.Body.Close() }()
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Microsoft Connect Test", string(b))

	testsuite.IsDestroyed(t, pool)
}

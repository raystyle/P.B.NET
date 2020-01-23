package testproxy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestPoolAndManager(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pool, manager := PoolAndManager(t)

	// test balance
	balance, err := pool.Get(TagBalance)
	require.NoError(t, err)

	// test http client
	transport := new(http.Transport)
	balance.HTTP(transport)
	testsuite.HTTPClient(t, transport, "localhost")

	testsuite.IsDestroyed(t, pool)
	require.NoError(t, manager.Close())
	testsuite.IsDestroyed(t, manager)
}

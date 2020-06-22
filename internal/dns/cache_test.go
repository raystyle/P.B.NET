package dns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const testCacheDomain = "github.com"

var (
	testExpectIPv4 = []string{"1.1.1.1"}
	testExpectIPv6 = []string{"2f0c::1"}
)

func testUpdateCache(client *Client, domain string) {
	client.updateCache(domain, TypeIPv4, testExpectIPv4)
	client.updateCache(domain, TypeIPv6, testExpectIPv6)
}

func TestClientCache(t *testing.T) {
	client := NewClient(nil, nil)

	t.Run("get expire time", func(t *testing.T) {
		e := client.GetCacheExpireTime()
		require.Equal(t, defaultCacheExpireTime, e)
	})

	t.Run("set expire time", func(t *testing.T) {
		const expire = 10 * time.Minute

		err := client.SetCacheExpireTime(expire)
		require.NoError(t, err)

		e := client.GetCacheExpireTime()
		require.Equal(t, expire, e)
	})

	t.Run("set invalid expire time", func(t *testing.T) {
		err := client.SetCacheExpireTime(3 * time.Hour)
		require.Equal(t, ErrInvalidExpireTime, err)
	})

	t.Run("update", func(t *testing.T) {
		// query empty cache, then create it
		result := client.queryCache(testCacheDomain, TypeIPv4)
		require.Empty(t, result)

		// <security> update doesn't exist domain
		testUpdateCache(client, "a")
	})

	t.Run("query exist cache", func(t *testing.T) {
		testUpdateCache(client, testCacheDomain)

		result := client.queryCache(testCacheDomain, TypeIPv4)
		require.Equal(t, testExpectIPv4, result)
		result = client.queryCache(testCacheDomain, TypeIPv6)
		require.Equal(t, testExpectIPv6, result)
	})

	t.Run("flush cache", func(t *testing.T) {
		testUpdateCache(client, testCacheDomain)

		client.FlushCache()

		result := client.queryCache(testCacheDomain, TypeIPv4)
		require.Empty(t, result)
	})
}

func TestClientCacheAboutExpire(t *testing.T) {
	// make DNS client
	client := NewClient(nil, nil)
	client.expire = 10 * time.Millisecond
	// query empty cache, then create it
	result := client.queryCache(testCacheDomain, TypeIPv4)
	require.Empty(t, result)
	// update cache
	testUpdateCache(client, testCacheDomain)
	// expire
	time.Sleep(50 * time.Millisecond)
	// clean cache
	result = client.queryCache(testCacheDomain, TypeIPv4)
	require.Empty(t, result)
}

func TestClientCacheAboutType(t *testing.T) {
	// make DNS client
	client := NewClient(nil, nil)
	// query empty cache, then create it
	result := client.queryCache(testCacheDomain, TypeIPv4)
	require.Empty(t, result)
	// update cache
	testUpdateCache(client, testCacheDomain)
	// query invalid type
	result = client.queryCache(testCacheDomain, "invalid type")
	require.Empty(t, result)
}

func TestClient_queryCache_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const domain = "test.com"

	ipv4 := []string{"1.1.1.1", "1.0.0.1"}
	ipv6 := []string{"240c::1111", "240c::1001"}

	t.Run("part", func(t *testing.T) {
		client := NewClient(nil, nil)

		init := func() {
			// must query first for create cache structure
			// update cache will not create it if domain is not exist.
			cache := client.queryCache(domain, TypeIPv4)
			require.Empty(t, cache)
			cache = client.queryCache(domain, TypeIPv6)
			require.Empty(t, cache)

			client.updateCache(domain, TypeIPv4, ipv4)
			client.updateCache(domain, TypeIPv6, ipv6)
		}
		ipv4 := func() {
			cache := client.queryCache(domain, TypeIPv4)
			require.Equal(t, ipv4, cache)
		}
		ipv6 := func() {
			cache := client.queryCache(domain, TypeIPv6)
			require.Equal(t, ipv6, cache)
		}
		cleanup := func() {
			client.FlushCache()
		}
		testsuite.RunParallel(100, init, cleanup, ipv4, ipv6)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = NewClient(nil, nil)

			// must query first for create cache structure
			// update cache will not create it if domain is not exist.
			cache := client.queryCache(domain, TypeIPv4)
			require.Empty(t, cache)
			cache = client.queryCache(domain, TypeIPv6)
			require.Empty(t, cache)

			client.updateCache(domain, TypeIPv4, ipv4)
			client.updateCache(domain, TypeIPv6, ipv6)
		}
		ipv4 := func() {
			cache := client.queryCache(domain, TypeIPv4)
			require.Equal(t, ipv4, cache)
		}
		ipv6 := func() {
			cache := client.queryCache(domain, TypeIPv6)
			require.Equal(t, ipv6, cache)
		}
		testsuite.RunParallel(100, init, nil, ipv4, ipv6)

		testsuite.IsDestroyed(t, client)
	})
}

func TestClient_updateCache_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	const domain = "test.com"

	ipv4 := []string{"1.1.1.1", "1.0.0.1"}
	ipv6 := []string{"240c::1111", "240c::1001"}

	t.Run("part", func(t *testing.T) {
		client := NewClient(nil, nil)

		init := func() {
			// must query first for create cache structure
			// update cache will not create it if domain is not exist.
			cache := client.queryCache(domain, TypeIPv4)
			require.Empty(t, cache)
			cache = client.queryCache(domain, TypeIPv6)
			require.Empty(t, cache)
		}
		updateIPv4 := func() {
			client.updateCache(domain, TypeIPv4, ipv4)
		}
		updateIPv6 := func() {
			client.updateCache(domain, TypeIPv6, ipv6)
		}
		cleanup := func() {
			cache := client.queryCache(domain, TypeIPv4)
			require.Equal(t, ipv4, cache)
			cache = client.queryCache(domain, TypeIPv6)
			require.Equal(t, ipv6, cache)

			client.FlushCache()
		}
		testsuite.RunParallel(100, init, cleanup, updateIPv4, updateIPv6)

		testsuite.IsDestroyed(t, client)
	})

	t.Run("whole", func(t *testing.T) {
		var client *Client

		init := func() {
			client = NewClient(nil, nil)

			// must query first for create cache structure
			// update cache will not create it if domain is not exist.
			cache := client.queryCache(domain, TypeIPv4)
			require.Empty(t, cache)
			cache = client.queryCache(domain, TypeIPv6)
			require.Empty(t, cache)
		}
		updateIPv4 := func() {
			client.updateCache(domain, TypeIPv4, ipv4)
		}
		updateIPv6 := func() {
			client.updateCache(domain, TypeIPv6, ipv6)
		}
		cleanup := func() {
			cache := client.queryCache(domain, TypeIPv4)
			require.Equal(t, ipv4, cache)
			cache = client.queryCache(domain, TypeIPv6)
			require.Equal(t, ipv6, cache)
		}
		testsuite.RunParallel(100, init, cleanup, updateIPv4, updateIPv6)

		testsuite.IsDestroyed(t, client)
	})
}

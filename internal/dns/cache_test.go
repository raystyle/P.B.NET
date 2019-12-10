package dns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/options"
)

const (
	testCacheDomain = "github.com"
)

var (
	testExpectIPv4 = []string{"1.1.1.1"}
	testExpectIPv6 = []string{"2f0c::1"}
)

func testUpdateCache(client *Client, domain string) {
	client.updateCache(domain, TypeIPv4, testExpectIPv4)
	client.updateCache(domain, TypeIPv6, testExpectIPv6)
}

func TestClientCache(t *testing.T) {
	// make DNS client
	client := NewClient(nil)

	// get cache expire time
	require.Equal(t, options.DefaultDNSCacheExpireTime, client.GetCacheExpireTime())

	// set cache expire time
	const expire = 10 * time.Minute
	require.NoError(t, client.SetCacheExpireTime(expire))
	require.Equal(t, expire, client.GetCacheExpireTime())

	// set invalid cache expire time
	require.Equal(t, ErrInvalidExpireTime, client.SetCacheExpireTime(3*time.Hour))

	// query empty cache, then create it
	result := client.queryCache(testCacheDomain, TypeIPv4)
	require.Equal(t, 0, len(result))

	// update cache
	testUpdateCache(client, testCacheDomain)

	// <security> update doesn't exists domain
	testUpdateCache(client, "a")

	// query exist cache
	result = client.queryCache(testCacheDomain, TypeIPv4)
	require.Equal(t, testExpectIPv4, result)
	result = client.queryCache(testCacheDomain, TypeIPv6)
	require.Equal(t, testExpectIPv6, result)

	// flush cache
	client.FlushCache()
	result = client.queryCache(testCacheDomain, TypeIPv4)
	require.Equal(t, 0, len(result))
}

func TestClientCacheAboutExpire(t *testing.T) {
	// make DNS client
	client := NewClient(nil)
	client.expire = 10 * time.Millisecond
	// query empty cache, then create it
	result := client.queryCache(testCacheDomain, TypeIPv4)
	require.Equal(t, 0, len(result))
	// update cache
	testUpdateCache(client, testCacheDomain)
	// expire
	time.Sleep(50 * time.Millisecond)
	// clean cache
	result = client.queryCache(testCacheDomain, TypeIPv4)
	require.Equal(t, 0, len(result))
}

func TestClientCacheAboutType(t *testing.T) {
	// make DNS client
	client := NewClient(nil)
	// query empty cache, then create it
	result := client.queryCache(testCacheDomain, TypeIPv4)
	require.Equal(t, 0, len(result))
	// update cache
	testUpdateCache(client, testCacheDomain)
	// query invalid type
	result = client.queryCache(testCacheDomain, "invalid type")
	require.Equal(t, 0, len(result))
}

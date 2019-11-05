package timesync

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite/testdns"
)

func TestHTTPClient_Query(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	NewHTTP(context.Background(), pool, dnsClient)
}

func TestTestHTTP(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	require.NoError(t, TestHTTP(b))
}

func TestGetHeaderDate(t *testing.T) {
	client := &http.Client{
		Transport: new(http.Transport),
		Timeout:   10 * time.Second,
	}
	r, err := http.NewRequest(http.MethodGet, "http://test-ipv6.com/", nil)
	require.NoError(t, err)
	now, err := getHeaderDate(r, client)
	require.NoError(t, err)
	t.Log(now)

	// https
	r, err = http.NewRequest(http.MethodGet, "https://cloudflare-dns.com/", nil)
	require.NoError(t, err)
	now, err = getHeaderDate(r, client)
	require.NoError(t, err)
	t.Log(now)

	// failed to query date
	r, err = http.NewRequest(http.MethodGet, "http://asdasd1516ads.com/", nil)
	require.NoError(t, err)
	_, err = getHeaderDate(r, client)
	require.Error(t, err)
}

func TestHTTPOptions(t *testing.T) {

}

package timesync

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/testsuite/testdns"
	"project/internal/testsuite/testproxy"
)

func TestHTTPClient_Query(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	HTTP := NewHTTP(context.Background(), pool, dnsClient)
	b, err := ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)
	require.NoError(t, HTTP.Import(b))

	// simple query
	now, optsErr, err := HTTP.Query()
	require.NoError(t, err)
	require.False(t, optsErr)
	t.Log("now(HTTP) simple:", now.Local())

	// http
	HTTP.Request.URL = "http://test-ipv6.com/"
	now, optsErr, err = HTTP.Query()
	require.NoError(t, err)
	require.False(t, optsErr)
	t.Log("now(HTTP) http:", now.Local())

	// with proxy
	HTTP.ProxyTag = testproxy.TagBalance
	HTTP.Request.URL = "http://test-ipv6.com:80/"
	now, optsErr, err = HTTP.Query()
	require.NoError(t, err)
	require.False(t, optsErr)
	t.Log("now(HTTP): with proxy", now.Local())
}

func TestHTTPClient_Query_Failed(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	HTTP := NewHTTP(context.Background(), pool, dnsClient)
	b, err := ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)
	require.NoError(t, HTTP.Import(b))

	// invalid request
	HTTP.Request.Post = "foo data"
	_, optsErr, err := HTTP.Query()
	require.Error(t, err)
	require.True(t, optsErr)

	HTTP.Request.Post = ""

	// invalid transport
	HTTP.Transport.TLSClientConfig.RootCAs = []string{"foo data"}
	_, optsErr, err = HTTP.Query()
	require.Error(t, err)
	require.True(t, optsErr)

	HTTP.Transport.TLSClientConfig.RootCAs = nil

	// doesn't exist proxy
	HTTP.ProxyTag = "foo proxy"
	_, optsErr, err = HTTP.Query()
	require.Error(t, err)
	require.True(t, optsErr)

	HTTP.ProxyTag = ""

	// invalid domain name
	HTTP.Request.URL = "http://asdasd1516ads.com/"
	_, optsErr, err = HTTP.Query()
	require.Error(t, err)
	require.True(t, optsErr)

	// all failed
	HTTP.Request.URL = "https://github.com:8989/"
	HTTP.Timeout = time.Second
	_, optsErr, err = HTTP.Query()
	require.Error(t, err)
	require.False(t, optsErr)
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
	b, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	require.NoError(t, TestHTTP(b))
	HTTP := new(HTTP)
	require.NoError(t, HTTP.Import(b))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: 15 * time.Second, actual: HTTP.Timeout},
		{expected: "balance", actual: HTTP.ProxyTag},
		{expected: "http://abc.com/", actual: HTTP.Request.URL},
		{expected: 2, actual: HTTP.Transport.MaxIdleConns},
		{expected: dns.ModeSystem, actual: HTTP.DNSOpts.Mode},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}

	// export
	export := HTTP.Export()
	require.NotEqual(t, 0, len(export))
	t.Log(string(export))
	require.NoError(t, HTTP.Import(export))
}

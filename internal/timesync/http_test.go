package timesync

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/patch/monkey"
	"project/internal/testsuite"
	"project/internal/testsuite/testcert"
	"project/internal/testsuite/testdns"
	"project/internal/testsuite/testproxy"
)

func TestHTTPClient_Query(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()

	t.Run("https", func(t *testing.T) {
		HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)

		b, err := ioutil.ReadFile("testdata/http.toml")
		require.NoError(t, err)
		require.NoError(t, HTTP.Import(b))

		now, optsErr, err := HTTP.Query()
		require.NoError(t, err)
		require.False(t, optsErr)
		t.Log("now(HTTPS):", now.Local())

		testsuite.IsDestroyed(t, HTTP)
	})

	t.Run("http with proxy", func(t *testing.T) {
		HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)

		HTTP.ProxyTag = testproxy.TagBalance
		HTTP.Request.URL = "http://ds.vm3.test-ipv6.com/"

		now, optsErr, err := HTTP.Query()
		require.NoError(t, err)
		require.False(t, optsErr)
		t.Log("now(HTTP): with proxy", now.Local())

		testsuite.IsDestroyed(t, HTTP)
	})
}

func TestHTTPClient_Query_Failed(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()

	newHTTP := func(t *testing.T) *HTTP {
		HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)
		HTTP.Request.URL = "test.com"
		HTTP.Transport.TLSClientConfig.CertPool = testcert.CertPool(t)
		return HTTP
	}

	t.Run("invalid request", func(t *testing.T) {
		HTTP := newHTTP(t)

		HTTP.Request.Post = "foo data"

		_, optsErr, err := HTTP.Query()
		require.Error(t, err)
		require.True(t, optsErr)

		testsuite.IsDestroyed(t, HTTP)
	})

	t.Run("invalid transport", func(t *testing.T) {
		HTTP := newHTTP(t)

		HTTP.Transport.TLSClientConfig.RootCAs = []string{"foo data"}

		_, optsErr, err := HTTP.Query()
		require.Error(t, err)
		require.True(t, optsErr)

		testsuite.IsDestroyed(t, HTTP)
	})

	t.Run("doesn't exist proxy", func(t *testing.T) {
		HTTP := newHTTP(t)

		HTTP.ProxyTag = "foo proxy"

		_, optsErr, err := HTTP.Query()
		require.Error(t, err)
		require.True(t, optsErr)

		testsuite.IsDestroyed(t, HTTP)
	})

	t.Run("invalid domain name", func(t *testing.T) {
		HTTP := newHTTP(t)

		HTTP.Request.URL = "http://test/"

		_, optsErr, err := HTTP.Query()
		require.Error(t, err)
		require.True(t, optsErr)

		testsuite.IsDestroyed(t, HTTP)
	})

	t.Run("all failed", func(t *testing.T) {
		HTTP := newHTTP(t)

		HTTP.Request.URL = "https://github.com:8989/"
		HTTP.Timeout = time.Second

		_, optsErr, err := HTTP.Query()
		require.Error(t, err)
		require.False(t, optsErr)

		testsuite.IsDestroyed(t, HTTP)
	})
}

func TestGetHeaderDate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := &http.Client{Transport: new(http.Transport)}

	t.Run("http", func(t *testing.T) {
		const url = "http://ds.vm3.test-ipv6.com/"
		r, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		now, err := getHeaderDate(r, client)
		require.NoError(t, err)
		t.Log(now.Local())
	})

	t.Run("https", func(t *testing.T) {
		const url = "https://cloudflare-dns.com/"
		r, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		now, err := getHeaderDate(r, client)
		require.NoError(t, err)
		t.Log(now.Local())
	})

	t.Run("failed to query date", func(t *testing.T) {
		const url = "http://test/"
		r, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)

		_, err = getHeaderDate(r, client)
		require.Error(t, err)
	})

	t.Run("failed to parse date", func(t *testing.T) {
		const url = "http://ds.vm3.test-ipv6.com/"

		// patch
		r, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		patch := func(_ string) (time.Time, error) {
			return time.Time{}, monkey.Error
		}
		pg := monkey.Patch(http.ParseTime, patch)
		defer pg.Unpatch()

		_, err = getHeaderDate(r, client)
		monkey.IsMonkeyError(t, err)
	})

	t.Run("system time changed", func(t *testing.T) {
		const url = "http://ds.vm3.test-ipv6.com/"

		// patch
		r, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		patch := func(_ time.Time) time.Duration {
			return time.Minute
		}
		pg := monkey.Patch(time.Since, patch)
		defer pg.Unpatch()

		_, err = getHeaderDate(r, client)
		require.NoError(t, err)
	})
}

func TestHTTPOptions(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/http_opts.toml")
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
		{expected: "http://test.com/", actual: HTTP.Request.URL},
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

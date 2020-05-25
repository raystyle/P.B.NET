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

func TestHTTP_Query(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("https", func(t *testing.T) {
		HTTP := NewHTTP(context.Background(), certPool, proxyPool, dnsClient)

		data, err := ioutil.ReadFile("testdata/http.toml")
		require.NoError(t, err)
		err = HTTP.Import(data)
		require.NoError(t, err)

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

func TestHTTP_Query_Failed(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

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

		_, _, err := HTTP.Query()
		require.Error(t, err)

		testsuite.IsDestroyed(t, HTTP)
	})
}

func TestGetDate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := &http.Client{Transport: new(http.Transport)}
	defer client.CloseIdleConnections()

	t.Run("http", func(t *testing.T) {
		const url = "http://ds.vm3.test-ipv6.com/"

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		now, err := getDate(req, client)
		require.NoError(t, err)

		t.Log(now.Local())
	})

	t.Run("https", func(t *testing.T) {
		const url = "https://cloudflare-dns.com/"

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		now, err := getDate(req, client)
		require.NoError(t, err)

		t.Log(now.Local())
	})

	t.Run("failed to query date", func(t *testing.T) {
		const url = "http://test/"

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)

		_, err = getDate(req, client)
		require.Error(t, err)
	})

	t.Run("failed to parse date", func(t *testing.T) {
		const url = "http://ds.vm3.test-ipv6.com/"

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		patch := func(string) (time.Time, error) {
			return time.Time{}, monkey.Error
		}
		pg := monkey.Patch(http.ParseTime, patch)
		defer pg.Unpatch()

		_, err = getDate(req, client)
		monkey.IsMonkeyError(t, err)
	})

	t.Run("system time changed", func(t *testing.T) {
		const url = "http://ds.vm3.test-ipv6.com/"

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		patch := func(time.Time) time.Duration {
			return time.Minute
		}
		pg := monkey.Patch(time.Since, patch)
		defer pg.Unpatch()

		_, err = getDate(req, client)
		require.NoError(t, err)
	})
}

func TestHTTPOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)

	err = TestHTTP(data)
	require.NoError(t, err)

	HTTP := new(HTTP)
	err = HTTP.Import(data)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, HTTP)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: 15 * time.Second, actual: HTTP.Timeout},
		{expected: "balance", actual: HTTP.ProxyTag},
		{expected: "http://test.com/", actual: HTTP.Request.URL},
		{expected: 2, actual: HTTP.Transport.MaxIdleConns},
		{expected: dns.ModeSystem, actual: HTTP.DNSOpts.Mode},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}

	// export
	export := HTTP.Export()
	require.NotEmpty(t, export)
	t.Log(string(export))

	err = HTTP.Import(export)
	require.NoError(t, err)
}

package httpproxy

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	server := testGenerateServer(t)
	err := server.ListenAndServe("localhost:0", 0)
	require.NoError(t, err)
	defer func() {
		err = server.Stop()
		require.NoError(t, err)
	}()
	httpProxy, err := NewClient("http://admin:123456@" + server.Addr())
	require.NoError(t, err)
	transport := &http.Transport{}
	httpProxy.HTTP(transport)
	client := http.Client{
		Transport: transport,
	}
	get := func(url string) {
		resp, err := client.Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		_, err = ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
	}
	get("https://github.com/")
	get("http://github.com/")
	get("http://admin:123456@" + server.Addr())
	// test other
	_, err = httpProxy.Dial("", "")
	require.Equal(t, err, ErrNotSupportDial)
	_, err = httpProxy.DialContext(nil, "", "")
	require.Equal(t, err, ErrNotSupportDial)
	_, err = httpProxy.DialTimeout("", "", 0)
	require.Equal(t, err, ErrNotSupportDial)
	t.Log(httpProxy.Info())
}

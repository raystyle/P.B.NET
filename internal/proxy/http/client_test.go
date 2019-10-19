package http

import (
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testutil"
)

func TestClient(t *testing.T) {
	server := testGenerateServer(t)
	require.NoError(t, server.ListenAndServe("localhost:0"))
	defer func() {
		require.NoError(t, server.Close())
		testutil.IsDestroyed(t, server, 2)
	}()
	client, err := NewClient("http://admin:123456@" + server.Addr())
	require.NoError(t, err)
	wg := sync.WaitGroup{}
	get := func(url string) {
		defer wg.Done()
		transport := &http.Transport{}
		client.HTTP(transport)
		resp, err := (&http.Client{Transport: transport}).Get(url)
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()
		transport.CloseIdleConnections()
		transport.Proxy = nil
	}
	wg.Add(3)
	go get("http://github.com/")
	go get("https://github.com/")
	go get("https://github.com/")
	// test other
	_, err = client.Dial("", "")
	require.Equal(t, err, ErrNotSupportDial)
	_, err = client.DialContext(nil, "", "")
	require.Equal(t, err, ErrNotSupportDial)
	_, err = client.DialTimeout("", "", 0)
	require.Equal(t, err, ErrNotSupportDial)
	t.Log(client.Info())
	wg.Wait()
	testutil.IsDestroyed(t, client, 2)
}

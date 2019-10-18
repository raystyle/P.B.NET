package http

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
	client, err := NewClient("http://admin:123456@" + server.Addr())
	require.NoError(t, err)
	get := func(url string) {
		transport := &http.Transport{}
		client.HTTP(transport)
		resp, err := (&http.Client{Transport: transport}).Get(url)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		_, err = ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
	}
	get("https://github.com/")
	get("http://github.com/")
	get("http://admin:123456@" + server.Addr())
	// test other
	_, err = client.Dial("", "")
	require.Equal(t, err, ErrNotSupportDial)
	_, err = client.DialContext(nil, "", "")
	require.Equal(t, err, ErrNotSupportDial)
	_, err = client.DialTimeout("", "", 0)
	require.Equal(t, err, ErrNotSupportDial)
	t.Log(client.Info())
	t.Log(client.Mode())
}

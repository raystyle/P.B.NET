package timesync

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryHTTPServer(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "https://cloudflare-dns.com", nil)
	require.NoError(t, err)
	client := http.Client{}
	_time, err := queryHTTPServer(r, &client)
	require.NoError(t, err)
	t.Log(_time)
	// query failed
	r, err = http.NewRequest(http.MethodGet, "http://asdasd1516ads.com", nil)
	require.NoError(t, err)
	_, err = queryHTTPServer(r, &client)
	require.Error(t, err)
}

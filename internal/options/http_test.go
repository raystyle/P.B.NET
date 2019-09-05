package options

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultHTTP(t *testing.T) {
	// request
	request, err := new(HTTPRequest).Apply()
	require.NoError(t, err)
	require.NotNil(t, request)
	// transport
	transport, err := new(HTTPTransport).Apply()
	require.NoError(t, err)
	require.NotNil(t, transport)
	// server
	server, err := new(HTTPServer).Apply()
	require.NoError(t, err)
	require.NotNil(t, server)
}

func TestCopyHTTPHeader(t *testing.T) {
	h := http.Header{}
	h.Add("aa", "aa")
	h2 := CopyHTTPHeader(h)
	require.Equal(t, h, h2)
}

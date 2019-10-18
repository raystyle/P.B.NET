package timesync

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPClient_Query(t *testing.T) {

}

func testNewHTTPClient(t *testing.T) *HTTPClient {
	b, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	client, err := NewHTTPClient(b)
	require.NoError(t, err)
	return client
}

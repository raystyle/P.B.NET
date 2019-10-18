package timesync

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNTPClient_Query(t *testing.T) {

}

func testNewNTPClient(t *testing.T) *NTPClient {
	b, err := ioutil.ReadFile("testdata/ntp.toml")
	require.NoError(t, err)
	client, err := NewNTPClient(b)
	require.NoError(t, err)
	return client
}

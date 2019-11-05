package timesync

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNTPClient_Query(t *testing.T) {

}

func TestTestNTP(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/ntp.toml")
	require.NoError(t, err)
	require.NoError(t, TestNTP(b))
}

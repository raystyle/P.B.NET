package testdata

import (
	"io/ioutil"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
)

func Proxy_Clients(t *testing.T) map[string]*proxyclient.Client {
	c := make(map[string]*proxyclient.Client)
	b, err := ioutil.ReadFile("../config/proxyclient.toml")
	require.Nil(t, err, err)
	err = toml.Unmarshal(b, &c)
	require.Nil(t, err, err)
	return c
}

func DNS_Clients(t *testing.T) map[string]*dnsclient.Client {
	c := make(map[string]*dnsclient.Client)
	b, err := ioutil.ReadFile("../config/dnsclient.toml")
	require.Nil(t, err, err)
	err = toml.Unmarshal(b, &c)
	require.Nil(t, err, err)
	return c
}

func Timesync_Client(t *testing.T) map[string]*timesync.Client {
	// create time sync client
	timesync_clients := make(map[string]*timesync.Client)
	timesync_clients["test_http"] = &timesync.Client{
		Mode:    timesync.HTTP,
		Address: "https://www.baidu.com/",
	}
	timesync_clients["test_ntp"] = &timesync.Client{
		Mode:    timesync.NTP,
		Address: "pool.ntp.org:123",
	}
	return timesync_clients
}

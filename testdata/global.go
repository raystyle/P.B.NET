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
	b, err := ioutil.ReadFile("../config/global/proxyclient.toml")
	require.Nil(t, err, err)
	err = toml.Unmarshal(b, &c)
	require.Nil(t, err, err)
	return c
}

func DNS_Clients(t *testing.T) map[string]*dnsclient.Client {
	c := make(map[string]*dnsclient.Client)
	b, err := ioutil.ReadFile("../config/global/dnsclient.toml")
	require.Nil(t, err, err)
	err = toml.Unmarshal(b, &c)
	require.Nil(t, err, err)
	return c
}

func Timesync_Client(t *testing.T) map[string]*timesync.Client {
	c := make(map[string]*timesync.Client)
	b, err := ioutil.ReadFile("../config/global/timesync.toml")
	require.Nil(t, err, err)
	err = toml.Unmarshal(b, &c)
	require.Nil(t, err, err)
	return c
}

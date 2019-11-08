package testdata

import (
	"io/ioutil"

	"github.com/stretchr/testify/require"

	"project/internal/messages"
	"project/internal/xnet"
)

func Listeners(t require.TestingT) []*messages.Listener {
	var listeners []*messages.Listener
	config, err := ioutil.ReadFile("../internal/xnet/testdata/tls.toml")
	require.NoError(t, err)
	listener := &messages.Listener{
		Tag:    "tls",
		Mode:   xnet.ModeTLS,
		Config: config,
	}
	listeners = append(listeners, listener)
	config, err = ioutil.ReadFile("../internal/xnet/testdata/light.toml")
	require.NoError(t, err)
	listener = &messages.Listener{
		Tag:    "light",
		Mode:   xnet.ModeLight,
		Config: config,
	}
	listeners = append(listeners, listener)
	return listeners
}

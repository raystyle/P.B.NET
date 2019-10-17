package testdata

import (
	"io/ioutil"

	"github.com/stretchr/testify/require"
	
	"project/internal/messages"
	"project/internal/xnet"
)

func Listeners(t require.TestingT) []*messages.Listener {
	var ls []*messages.Listener
	b, err := ioutil.ReadFile("../config/listener/tls.toml")
	require.NoError(t, err)
	l := &messages.Listener{
		Tag:    "tls",
		Mode:   xnet.TLS,
		Config: b,
	}
	ls = append(ls, l)
	b, err = ioutil.ReadFile("../config/listener/light.toml")
	require.NoError(t, err)
	l = &messages.Listener{
		Tag:    "light",
		Mode:   xnet.Light,
		Config: b,
	}
	ls = append(ls, l)
	return ls
}

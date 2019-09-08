package testdata

import (
	"io/ioutil"

	"github.com/stretchr/testify/require"

	"project/internal/config"
	"project/internal/xnet"
)

func Listeners(t require.TestingT) []*config.Listener {
	var ls []*config.Listener
	b, err := ioutil.ReadFile("../config/listener/tls.toml")
	require.NoError(t, err)
	l := &config.Listener{
		Tag:    "tls",
		Mode:   xnet.TLS,
		Config: b,
	}
	ls = append(ls, l)
	b, err = ioutil.ReadFile("../config/listener/light.toml")
	require.NoError(t, err)
	l = &config.Listener{
		Tag:    "light",
		Mode:   xnet.LIGHT,
		Config: b,
	}
	ls = append(ls, l)
	return ls
}

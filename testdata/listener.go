package testdata

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/messages"
	"project/internal/xnet"
)

func Listeners(t *testing.T) []*messages.Listener {
	var ls []*messages.Listener
	b, err := ioutil.ReadFile("../config/listener/tls.toml")
	require.Nil(t, err, err)
	l := &messages.Listener{
		Tag:    "tls",
		Mode:   xnet.TLS,
		Config: b,
	}
	ls = append(ls, l)
	b, err = ioutil.ReadFile("../config/listener/light.toml")
	require.Nil(t, err, err)
	l = &messages.Listener{
		Tag:    "light",
		Mode:   xnet.LIGHT,
		Config: b,
	}
	ls = append(ls, l)
	return ls
}

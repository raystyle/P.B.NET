package testdata

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/xnet"
)

func Listeners(t *testing.T) map[string]*xnet.Config {
	l := make(map[string]*xnet.Config)
	b, err := ioutil.ReadFile("../config/listener/tls.toml")
	require.Nil(t, err, err)
	c := &xnet.Config{
		Mode:   xnet.TLS,
		Config: b,
	}
	l["tls"] = c
	b, err = ioutil.ReadFile("../config/listener/light.toml")
	require.Nil(t, err, err)
	c = &xnet.Config{
		Mode:   xnet.LIGHT,
		Config: b,
	}
	l["light"] = c
	return l
}

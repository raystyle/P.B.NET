package testdata

import (
	"io/ioutil"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/config"
)

func Register(t require.TestingT) []*config.Bootstrap {
	var r []*config.Bootstrap
	// http
	b, err := ioutil.ReadFile("../config/bootstrap/http.toml")
	require.NoError(t, err)
	c := &config.Bootstrap{
		Tag:    "http",
		Mode:   bootstrap.ModeHTTP,
		Config: b,
	}
	r = append(r, c)
	// dns
	b, err = ioutil.ReadFile("../config/bootstrap/dns.toml")
	require.NoError(t, err)
	c = &config.Bootstrap{
		Tag:    "dns",
		Mode:   bootstrap.ModeDNS,
		Config: b,
	}
	r = append(r, c)
	// direct
	b, err = ioutil.ReadFile("../config/bootstrap/direct.toml")
	require.NoError(t, err)
	c = &config.Bootstrap{
		Tag:    "direct",
		Mode:   bootstrap.ModeDirect,
		Config: b,
	}
	r = append(r, c)
	return r
}

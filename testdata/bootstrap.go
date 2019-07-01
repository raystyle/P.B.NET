package testdata

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/config"
)

func Register(t *testing.T) []*config.Bootstrap {
	var r []*config.Bootstrap
	// http
	b, err := ioutil.ReadFile("../config/bootstrap/http.toml")
	require.Nil(t, err, err)
	c := &config.Bootstrap{
		Tag:    "http",
		Mode:   bootstrap.M_HTTP,
		Config: b,
	}
	r = append(r, c)
	// dns
	b, err = ioutil.ReadFile("../config/bootstrap/dns.toml")
	require.Nil(t, err, err)
	c = &config.Bootstrap{
		Tag:    "dns",
		Mode:   bootstrap.M_DNS,
		Config: b,
	}
	r = append(r, c)
	// direct
	b, err = ioutil.ReadFile("../config/bootstrap/direct.toml")
	require.Nil(t, err, err)
	c = &config.Bootstrap{
		Tag:    "direct",
		Mode:   bootstrap.M_DIRECT,
		Config: b,
	}
	r = append(r, c)
	return r
}

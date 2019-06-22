package testdata

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
)

func Register(t *testing.T) []*messages.Bootstrap {
	var r []*messages.Bootstrap
	// http
	b, err := ioutil.ReadFile("../config/bootstrap/http.toml")
	require.Nil(t, err, err)
	c := &messages.Bootstrap{
		Mode:   bootstrap.M_HTTP,
		Config: b,
	}
	r = append(r, c)
	// dns
	b, err = ioutil.ReadFile("../config/bootstrap/dns.toml")
	require.Nil(t, err, err)
	c = &messages.Bootstrap{
		Mode:   bootstrap.M_DNS,
		Config: b,
	}
	r = append(r, c)
	// direct
	b, err = ioutil.ReadFile("../config/bootstrap/direct.toml")
	require.Nil(t, err, err)
	c = &messages.Bootstrap{
		Mode:   bootstrap.M_DIRECT,
		Config: b,
	}
	r = append(r, c)
	return r
}

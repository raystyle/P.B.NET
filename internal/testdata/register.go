package testdata

import (
	"io/ioutil"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
)

func Register(t require.TestingT) []*messages.Bootstrap {
	var bootstraps []*messages.Bootstrap
	// http
	config, err := ioutil.ReadFile("../internal/bootstrap/testdata/http.toml")
	require.NoError(t, err)
	boot := &messages.Bootstrap{
		Tag:    "http",
		Mode:   bootstrap.ModeHTTP,
		Config: config,
	}
	bootstraps = append(bootstraps, boot)
	// dns
	config, err = ioutil.ReadFile("../internal/bootstrap/testdata/dns.toml")
	require.NoError(t, err)
	boot = &messages.Bootstrap{
		Tag:    "dns",
		Mode:   bootstrap.ModeDNS,
		Config: config,
	}
	bootstraps = append(bootstraps, boot)
	// direct
	config, err = ioutil.ReadFile("../internal/bootstrap/testdata/direct.toml")
	require.NoError(t, err)
	boot = &messages.Bootstrap{
		Tag:    "direct",
		Mode:   bootstrap.ModeDirect,
		Config: config,
	}
	bootstraps = append(bootstraps, boot)
	return bootstraps
}

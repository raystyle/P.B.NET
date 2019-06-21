package testdata

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
)

func Register(t *testing.T) []*bootstrap.Config {
	var registers []*bootstrap.Config
	// http
	b, err := ioutil.ReadFile("../config/bootstrap/http.toml")
	require.Nil(t, err, err)
	c := &bootstrap.Config{
		Mode:   bootstrap.M_HTTP,
		Config: b,
	}
	registers = append(registers, c)
	// dns
	b, err = ioutil.ReadFile("../config/bootstrap/dns.toml")
	require.Nil(t, err, err)
	c = &bootstrap.Config{
		Mode:   bootstrap.M_DNS,
		Config: b,
	}
	registers = append(registers, c)
	// direct
	b, err = ioutil.ReadFile("../config/bootstrap/direct.toml")
	require.Nil(t, err, err)
	c = &bootstrap.Config{
		Mode:   bootstrap.M_DIRECT,
		Config: b,
	}
	registers = append(registers, c)
	return registers
}

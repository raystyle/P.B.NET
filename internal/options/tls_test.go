package options

import (
	"io/ioutil"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
)

func TestTLSDefault(t *testing.T) {
	config, err := new(TLSConfig).Apply()
	require.NoError(t, err)
	require.NotNil(t, config)
}

func TestTLSUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("tls.toml")
	require.NoError(t, err)
	config := TLSConfig{}
	err = toml.Unmarshal(data, &config)
	require.NoError(t, err)
	tlsConfig, err := config.Apply()
	require.NoError(t, err)
	// check
	require.Equal(t, 2, len(tlsConfig.Certificates))
	require.Equal(t, 2, len(tlsConfig.RootCAs.Subjects()))
	require.Equal(t, 2, len(tlsConfig.ClientCAs.Subjects()))
	require.Equal(t, true, tlsConfig.InsecureSkipVerify)
	require.Equal(t, "h2", tlsConfig.NextProtos[0])
	require.Equal(t, "h2c", tlsConfig.NextProtos[1])
}

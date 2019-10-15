package options

import (
	"io/ioutil"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
)

func TestTLSDefault(t *testing.T) {
	tlsConfig, err := new(TLSConfig).Apply()
	require.NoError(t, err)
	// check
	require.Equal(t, 0, len(tlsConfig.Certificates))
	require.Nil(t, tlsConfig.RootCAs)
	require.Nil(t, tlsConfig.ClientCAs)
	require.Equal(t, 0, len(tlsConfig.NextProtos))
	require.Equal(t, false, tlsConfig.InsecureSkipVerify)
}

func TestTLSUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/tls.toml")
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
	require.Equal(t, "h2", tlsConfig.NextProtos[0])
	require.Equal(t, "h2c", tlsConfig.NextProtos[1])
	require.Equal(t, true, tlsConfig.InsecureSkipVerify)
}

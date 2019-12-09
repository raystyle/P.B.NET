package options

import (
	"crypto/tls"
	"io/ioutil"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert/certutil"
)

func TestTLSDefault(t *testing.T) {
	config, err := new(TLSConfig).Apply()
	require.NoError(t, err)
	// check
	require.Equal(t, 0, len(config.Certificates))
	require.Equal(t, 0, len(config.RootCAs.Subjects()))
	require.Equal(t, 0, len(config.ClientCAs.Subjects()))
	require.Equal(t, 0, len(config.NextProtos))
	require.Equal(t, false, config.InsecureSkipVerify)
}

func TestTLSUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/tls.toml")
	require.NoError(t, err)
	config := TLSConfig{}
	err = toml.Unmarshal(data, &config)
	require.NoError(t, err)
	tlsConfig, err := config.Apply()
	require.NoError(t, err)

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "test.com", actual: tlsConfig.ServerName},
		{expected: "h2", actual: tlsConfig.NextProtos[0]},
		{expected: "h2c", actual: tlsConfig.NextProtos[1]},
		{expected: uint16(tls.VersionTLS12), actual: tlsConfig.MinVersion},
		{expected: false, actual: tlsConfig.InsecureSkipVerify},
		{expected: true, actual: config.InsecureLoadFromSystem},
		{expected: 2, actual: len(tlsConfig.Certificates)},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}

	systemPool, err := certutil.SystemCertPool()
	require.NoError(t, err)
	require.Equal(t, 2+len(systemPool.Subjects()), len(tlsConfig.RootCAs.Subjects()))
	require.Equal(t, 2, len(tlsConfig.ClientCAs.Subjects()))
}

func TestTLSConfig_Apply_failed(t *testing.T) {
	// invalid certificates
	config := TLSConfig{}
	config.Certificates = append(config.Certificates, X509KeyPair{
		Cert: "foo data",
		Key:  "foo data",
	})
	_, err := config.Apply()
	require.Error(t, err)

	// invalid Root CAs
	config.Certificates = nil
	config.RootCAs = []string{"foo data"}
	_, err = config.Apply()
	require.Error(t, err)

	// invalid Client CAs
	config.RootCAs = nil
	config.ClientCAs = []string{"foo data"}
	_, err = config.Apply()
	require.Error(t, err)
}

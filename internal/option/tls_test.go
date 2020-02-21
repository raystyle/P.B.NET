package option

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert/certutil"
	"project/internal/patch/monkey"
	"project/internal/patch/toml"
)

func TestTLSDefault(t *testing.T) {
	config, err := new(TLSConfig).Apply()
	require.NoError(t, err)

	require.Nil(t, config.Certificates)
	require.Equal(t, 0, len(config.RootCAs.Subjects()))
	require.Nil(t, config.ClientCAs)
	require.Equal(t, tls.ClientAuthType(0), config.ClientAuth)
	require.Equal(t, "", config.ServerName)
	require.Nil(t, config.NextProtos)
	require.Equal(t, uint16(0), config.MaxVersion)
	require.Nil(t, config.CipherSuites)
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

	systemPool, err := certutil.SystemCertPool()
	require.NoError(t, err)

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: 2, actual: len(tlsConfig.Certificates)},
		{expected: 2 + len(systemPool.Subjects()), actual: len(tlsConfig.RootCAs.Subjects())},
		{expected: 2, actual: len(tlsConfig.ClientCAs.Subjects())},
		{expected: tls.ClientAuthType(4), actual: tlsConfig.ClientAuth},
		{expected: "test.com", actual: tlsConfig.ServerName},
		{expected: []string{"h2", "h2c"}, actual: tlsConfig.NextProtos},
		{expected: uint16(tls.VersionTLS11), actual: tlsConfig.MinVersion},
		{expected: uint16(tls.VersionTLS13), actual: tlsConfig.MaxVersion},
		{expected: []uint16{tls.TLS_RSA_WITH_AES_128_GCM_SHA256}, actual: tlsConfig.CipherSuites},
		{expected: false, actual: tlsConfig.InsecureSkipVerify},
		{expected: true, actual: config.InsecureLoadFromSystem},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

func TestTLSConfig_Apply(t *testing.T) {
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

	// failed to get system certificate pool
	config.InsecureLoadFromSystem = true
	pg := monkey.Patch(certutil.SystemCertPool, func() (*x509.CertPool, error) {
		return nil, errors.New("monkey error")
	})
	defer pg.Unpatch()
	_, err = config.Apply()
	require.EqualError(t, err, "failed to apply tls config: monkey error")
}

package option

import (
	"crypto/tls"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/toml"
	"project/internal/testsuite/testcert"
)

func TestTLSDefault(t *testing.T) {
	t.Run("client side", func(t *testing.T) {
		tlsConfig := TLSConfig{}

		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Equal(t, 0, len(config.Certificates))
			require.Equal(t, 0, len(config.RootCAs.Certs()))
			require.Equal(t, 0, len(config.ClientCAs.Certs()))
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Equal(t, testcert.PublicClientCertNum, len(config.Certificates))
			require.Equal(t, testcert.PublicRootCANum, len(config.RootCAs.Certs()))
			require.Equal(t, 0, len(config.ClientCAs.Certs()))
		})
	})

	t.Run("server side", func(t *testing.T) {
		tlsConfig := TLSConfig{ServerSide: true}

		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Equal(t, 0, len(config.Certificates))
			require.Equal(t, 0, len(config.RootCAs.Certs()))
			require.Equal(t, 0, len(config.ClientCAs.Certs()))
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Equal(t, 0, len(config.Certificates))
			require.Equal(t, 0, len(config.RootCAs.Certs()))
			require.Equal(t, testcert.PublicClientCANum, len(config.ClientCAs.Certs()))
		})
	})

	t.Run("common", func(t *testing.T) {
		config, err := new(TLSConfig).Apply()
		require.NoError(t, err)

		require.Equal(t, tls.ClientAuthType(0), config.ClientAuth)
		require.Zero(t, config.ServerName)
		require.Nil(t, config.NextProtos)
		require.Equal(t, uint16(0), config.MaxVersion)
		require.Nil(t, config.CipherSuites)
		require.Equal(t, false, config.InsecureSkipVerify)
	})
}

// the number of the certificate in testdata/tls.toml
const (
	testRootCANum      = 3
	testClientCANum    = 2
	testCertificateNum = 1
)

func TestTLSUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/tls.toml")
	require.NoError(t, err)
	tlsConfig := TLSConfig{}
	err = toml.Unmarshal(data, &tlsConfig)
	require.NoError(t, err)

	t.Run("client side", func(t *testing.T) {
		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Equal(t, testCertificateNum, len(config.Certificates))
			require.Equal(t, testRootCANum, len(config.RootCAs.Certs()))
			require.Equal(t, 0, len(config.ClientCAs.Certs()))
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			clientCertNum := testCertificateNum + testcert.PrivateClientCertNum
			require.Equal(t, clientCertNum, len(config.Certificates))
			rootCANum := testRootCANum + testcert.PrivateRootCANum
			require.Equal(t, rootCANum, len(config.RootCAs.Certs()))
			require.Equal(t, 0, len(config.ClientCAs.Certs()))
		})
	})

	t.Run("server side", func(t *testing.T) {
		tlsConfig.CertPool = nil
		tlsConfig.ServerSide = true

		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Equal(t, testCertificateNum, len(config.Certificates))
			require.Equal(t, 0, len(config.RootCAs.Certs()))
			require.Equal(t, testClientCANum, len(config.ClientCAs.Certs()))
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Equal(t, testCertificateNum, len(config.Certificates))
			require.Equal(t, 0, len(config.RootCAs.Certs()))
			clientCANum := testClientCANum + testcert.PrivateClientCANum
			require.Equal(t, clientCANum, len(config.ClientCAs.Certs()))
		})
	})

	t.Run("common", func(t *testing.T) {
		tlsConfig.CertPool = nil
		config, err := tlsConfig.Apply()
		require.NoError(t, err)

		testdata := [...]*struct {
			expected interface{}
			actual   interface{}
		}{
			{expected: tls.ClientAuthType(4), actual: config.ClientAuth},
			{expected: "test.com", actual: config.ServerName},
			{expected: []string{"h2", "h2c"}, actual: config.NextProtos},
			{expected: uint16(tls.VersionTLS11), actual: config.MinVersion},
			{expected: uint16(tls.VersionTLS13), actual: config.MaxVersion},
			{expected: []uint16{tls.TLS_RSA_WITH_AES_128_GCM_SHA256}, actual: config.CipherSuites},
			{expected: false, actual: config.InsecureSkipVerify},
		}
		for _, td := range testdata {
			require.Equal(t, td.expected, td.actual)
		}
	})
}

func TestTLSConfig_Apply(t *testing.T) {
	config := TLSConfig{}

	t.Run("invalid certificates", func(t *testing.T) {
		config.Certificates = append(config.Certificates, X509KeyPair{
			Cert: "foo data",
			Key:  "foo data",
		})
		_, err := config.Apply()
		require.Error(t, err)
	})

	t.Run("invalid Root CAs", func(t *testing.T) {
		config.Certificates = nil
		config.RootCAs = []string{"foo data"}
		_, err := config.Apply()
		require.Error(t, err)
	})

	t.Run("invalid Client CAs", func(t *testing.T) {
		config.ServerSide = true
		config.ClientCAs = []string{"foo data"}
		_, err := config.Apply()
		require.Error(t, err)
	})
}

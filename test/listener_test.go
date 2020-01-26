package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/messages"
	"project/internal/option"
	"project/internal/xnet"
)

func TestNodeListener(t *testing.T) {
	NODE := generateNodeAndTrust(t)
	defer NODE.Exit(nil)

	t.Run("QUIC", func(t *testing.T) {
		t.Parallel()

		// generate certificate
		pairs := ctrl.GetSelfCerts()
		opts := cert.Options{
			DNSNames:    []string{"localhost"},
			IPAddresses: []string{"127.0.0.1", "::1"},
		}
		caCert := pairs[0].Certificate
		caKey := pairs[0].PrivateKey
		pair, err := cert.Generate(caCert, caKey, &opts)
		require.NoError(t, err)

		// generate listener config
		listener := messages.Listener{
			Tag:     "quic",
			Mode:    xnet.ModeQUIC,
			Network: "udp",
			Address: "localhost:0",
		}
		c, k := pair.EncodeToPEM()
		listener.TLSConfig.Certificates = []option.X509KeyPair{
			{Cert: string(c), Key: string(k)},
		}

		err = NODE.AddListener(&listener)
		require.NoError(t, err)

		// use controller connect it
	})

	t.Run("Light", func(t *testing.T) {
		t.Parallel()

		listener := messages.Listener{
			Tag:     "light",
			Mode:    xnet.ModeLight,
			Network: "tcp",
			Address: "localhost:0",
		}

		err := NODE.AddListener(&listener)
		require.NoError(t, err)
	})

	t.Run("TLS", func(t *testing.T) {
		t.Parallel()

		// generate certificate
		pairs := ctrl.GetSelfCerts()
		opts := cert.Options{
			DNSNames:    []string{"localhost"},
			IPAddresses: []string{"127.0.0.1", "::1"},
		}
		caCert := pairs[0].Certificate
		caKey := pairs[0].PrivateKey
		pair, err := cert.Generate(caCert, caKey, &opts)
		require.NoError(t, err)

		// generate listener config
		listener := messages.Listener{
			Tag:     "tls",
			Mode:    xnet.ModeTLS,
			Network: "tcp",
			Address: "localhost:0",
		}
		c, k := pair.EncodeToPEM()
		listener.TLSConfig.Certificates = []option.X509KeyPair{
			{Cert: string(c), Key: string(k)},
		}

		err = NODE.AddListener(&listener)
		require.NoError(t, err)
	})
}

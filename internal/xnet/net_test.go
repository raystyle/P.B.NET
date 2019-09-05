package xnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/options"
)

func TestCheckPortString(t *testing.T) {
	err := CheckPortString("1234")
	require.NoError(t, err)
	err = CheckPortString("")
	require.Equal(t, ErrEmptyPort, err)
	err = CheckPortString("s")
	require.Error(t, err)
	err = CheckPortString("0")
	require.Error(t, err)
	err = CheckPortString("65536")
	require.Error(t, err)
}

func TestCheckPortInt(t *testing.T) {
	err := CheckPortInt(123)
	require.NoError(t, err)
	err = CheckPortInt(0)
	require.Equal(t, ErrInvalidPort, err)
	err = CheckPortInt(65536)
	require.Equal(t, ErrInvalidPort, err)
}

func TestCheckModeNetwork(t *testing.T) {
	err := CheckModeNetwork(TLS, "tcp")
	require.NoError(t, err)
	err = CheckModeNetwork(TLS, "udp")
	require.Equal(t, ErrMismatchedModeNetwork, err)
	err = CheckModeNetwork(LIGHT, "tcp")
	require.NoError(t, err)
	err = CheckModeNetwork(LIGHT, "udp")
	require.Equal(t, ErrMismatchedModeNetwork, err)
	err = CheckModeNetwork("", "")
	require.Equal(t, ErrEmptyMode, err)
	err = CheckModeNetwork(TLS, "")
	require.Equal(t, ErrEmptyNetwork, err)
	err = CheckModeNetwork("xxxx", "xxxx")
	require.Equal(t, ErrUnknownMode, err)
}

func TestListenAndDialTLS(t *testing.T) {
	cfg := &Config{
		Network: "tcp",
		Address: "localhost:0",
	}
	// add cert
	certCfg := &cert.Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	certificate, key, err := cert.Generate(nil, nil, certCfg)
	require.NoError(t, err)
	kp := options.X509KeyPair{Cert: string(certificate), Key: string(key)}
	cfg.TLSConfig.Certificates = append(cfg.TLSConfig.Certificates, kp)
	// Listen
	listener, err := Listen(TLS, cfg)
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		require.NoError(t, err)
		_, _ = conn.Write([]byte{0})
		_ = conn.Close()
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	cfg.Address = "localhost:" + port
	cfg.TLSConfig.RootCAs = append(cfg.TLSConfig.RootCAs, string(certificate))
	conn, err := Dial(TLS, cfg)
	require.NoError(t, err)
	_ = conn.Close()
}

func TestListenAndDialLight(t *testing.T) {
	cfg := &Config{
		Network: "tcp",
		Address: "localhost:0",
	}
	// Listen
	listener, err := Listen(LIGHT, cfg)
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		require.NoError(t, err)
		_, _ = conn.Write([]byte{0})
		_ = conn.Close()
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	cfg.Address = "localhost:" + port
	conn, err := Dial(LIGHT, cfg)
	require.NoError(t, err)
	_ = conn.Close()
}

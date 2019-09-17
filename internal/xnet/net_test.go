package xnet

import (
	"net"
	"sync"
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

func TestCheckPort(t *testing.T) {
	err := CheckPort(123)
	require.NoError(t, err)

	err = CheckPort(0)
	require.Error(t, err)
	require.Equal(t, "invalid port: 0", err.Error())
	err = CheckPort(65536)
	require.Error(t, err)
	require.Equal(t, "invalid port: 65536", err.Error())
}

func TestCheckModeNetwork(t *testing.T) {
	err := CheckModeNetwork(TLS, "tcp")
	require.NoError(t, err)
	err = CheckModeNetwork(QUIC, "udp")
	require.NoError(t, err)
	err = CheckModeNetwork(Light, "tcp")
	require.NoError(t, err)

	err = CheckModeNetwork(TLS, "udp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: tls udp", err.Error())
	err = CheckModeNetwork(QUIC, "tcp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: quic tcp", err.Error())
	err = CheckModeNetwork(Light, "udp")
	require.Error(t, err)
	require.Equal(t, "mismatched mode and network: light udp", err.Error())

	err = CheckModeNetwork("", "")
	require.Equal(t, ErrEmptyMode, err)
	err = CheckModeNetwork(TLS, "")
	require.Equal(t, ErrEmptyNetwork, err)

	err = CheckModeNetwork("xxxx", "xxxx")
	require.Error(t, err)
	require.Equal(t, "unknown mode: xxxx", err.Error())
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
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.NoError(t, err)
		_, err = conn.Write([]byte{0})
		require.NoError(t, err)
		_ = conn.Close()
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	cfg.Address = "localhost:" + port
	cfg.TLSConfig.RootCAs = append(cfg.TLSConfig.RootCAs, string(certificate))
	conn, err := Dial(TLS, cfg)
	require.NoError(t, err)
	_, err = conn.Write([]byte{0})
	require.NoError(t, err)
	_ = conn.Close()
	wg.Wait()
}

func TestListenAndDialQUIC(t *testing.T) {
	cfg := &Config{
		Network: "udp",
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
	listener, err := Listen(QUIC, cfg)
	require.NoError(t, err)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.NoError(t, err)
		_ = conn.Close()
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	cfg.Address = "localhost:" + port
	cfg.TLSConfig.RootCAs = append(cfg.TLSConfig.RootCAs, string(certificate))
	conn, err := Dial(QUIC, cfg)
	require.NoError(t, err)
	_, err = conn.Write([]byte{0})
	require.NoError(t, err)
	_ = conn.Close()
	wg.Wait()
}

func TestListenAndDialLight(t *testing.T) {
	cfg := &Config{
		Network: "tcp",
		Address: "localhost:0",
	}
	// Listen
	listener, err := Listen(Light, cfg)
	require.NoError(t, err)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.NoError(t, err)
		_, err = conn.Write([]byte{0})
		require.NoError(t, err)
		_ = conn.Close()
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	cfg.Address = "localhost:" + port
	conn, err := Dial(Light, cfg)
	require.NoError(t, err)
	_, err = conn.Write([]byte{0})
	require.NoError(t, err)
	_ = conn.Close()
	wg.Wait()
}

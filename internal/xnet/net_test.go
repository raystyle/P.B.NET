package xnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
	"project/internal/options"
)

func Test_Check_Port_str(t *testing.T) {
	err := Check_Port_str("1234")
	require.Nil(t, err, err)
	err = Check_Port_str("")
	require.Equal(t, ERR_EMPTY_PORT, err)
	err = Check_Port_str("s")
	require.NotNil(t, err)
	err = Check_Port_str("0")
	require.NotNil(t, err)
	err = Check_Port_str("65536")
	require.NotNil(t, err)
}

func Test_Check_Port_int(t *testing.T) {
	err := Check_Port_int(123)
	require.Nil(t, err, err)
	err = Check_Port_int(0)
	require.Equal(t, ERR_INVALID_PORT, err)
	err = Check_Port_int(65536)
	require.Equal(t, ERR_INVALID_PORT, err)
}

func Test_Check_Mode_Network(t *testing.T) {
	err := Check_Mode_Network(TLS, "tcp")
	require.Nil(t, err, err)
	err = Check_Mode_Network(TLS, "udp")
	require.Equal(t, ERR_MISMATCHED_MODE_NETWORK, err)
	err = Check_Mode_Network(LIGHT, "tcp")
	require.Nil(t, err, err)
	err = Check_Mode_Network(LIGHT, "udp")
	require.Equal(t, ERR_MISMATCHED_MODE_NETWORK, err)
	err = Check_Mode_Network("", "")
	require.Equal(t, ERR_EMPTY_MODE, err)
	err = Check_Mode_Network(TLS, "")
	require.Equal(t, ERR_EMPTY_NETWORK, err)
	err = Check_Mode_Network("asdasd", "adasdas")
	require.Equal(t, ERR_UNKNOWN_MODE, err)
}

func Test_Listen_And_Dial_TLS(t *testing.T) {
	c := &Config{
		Network: "tcp",
		Address: ":0",
	}
	// add cert
	cert_config := &cert.Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	certificate, key, err := cert.Generate(nil, nil, cert_config)
	require.Nil(t, err, err)
	kp := options.TLS_KeyPair{Cert: string(certificate), Key: string(key)}
	c.TLS_Config.Certificates = append(c.TLS_Config.Certificates, kp)
	// Listen
	listener, err := Listen(TLS, c)
	require.Nil(t, err, err)
	go func() {
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		_, _ = conn.Write([]byte{0})
		_ = conn.Close()
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.Nil(t, err, err)
	c.Address = "localhost:" + port
	c.TLS_Config.RootCAs = append(c.TLS_Config.RootCAs, string(certificate))
	conn, err := Dial(TLS, c)
	require.Nil(t, err, err)
	_ = conn.Close()
}

func Test_Listen_And_Dial_Light(t *testing.T) {
	c := &Config{
		Network: "tcp",
		Address: ":0",
	}
	// Listen
	listener, err := Listen(LIGHT, c)
	require.Nil(t, err, err)
	go func() {
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		_, _ = conn.Write([]byte{0})
		_ = conn.Close()
	}()
	// Dial
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.Nil(t, err, err)
	c.Address = "localhost:" + port
	conn, err := Dial(LIGHT, c)
	require.Nil(t, err, err)
	_ = conn.Close()
}

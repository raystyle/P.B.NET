package xtls

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
)

func TestXTLS(t *testing.T) {
	// generate cert
	certConfig := &cert.Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	c, k, err := cert.Generate(nil, nil, certConfig)
	require.NoError(t, err)
	tlsCert, err := tls.X509KeyPair(c, k)
	require.NoError(t, err)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}
	listener, err := Listen("tcp", "localhost:0", tlsConfig, 0)
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		require.NoError(t, err)
		write := func() {
			testdata := testGenerateTestdata()
			_, err = conn.Write(testdata)
			require.NoError(t, err)
			require.Equal(t, testGenerateTestdata(), testdata)
		}
		read := func() {
			data := make([]byte, 256)
			_, err = io.ReadFull(conn, data)
			require.NoError(t, err)
			require.Equal(t, testGenerateTestdata(), data)
		}
		read()
		write()
		write()
		read()
	}()
	// add cert to trust
	tlsConfig = &tls.Config{
		RootCAs: x509.NewCertPool(),
	}
	x509Cert, err := cert.Parse(c)
	require.NoError(t, err)
	tlsConfig.RootCAs.AddCert(x509Cert)
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	conn, err := Dial("tcp", "localhost:"+port, tlsConfig, 0)
	require.NoError(t, err)
	write := func() {
		testdata := testGenerateTestdata()
		_, err = conn.Write(testdata)
		require.NoError(t, err)
		require.Equal(t, testGenerateTestdata(), testdata)
	}
	read := func() {
		data := make([]byte, 256)
		_, err = io.ReadFull(conn, data)
		require.NoError(t, err)
		require.Equal(t, testGenerateTestdata(), data)
	}
	write()
	read()
	read()
	write()
}

func testGenerateTestdata() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

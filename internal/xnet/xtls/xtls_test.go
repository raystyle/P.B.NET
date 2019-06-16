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

func Test_xtls(t *testing.T) {
	// generate cert
	c, k, err := cert.Generate(nil, nil,
		[]string{"localhost"}, []string{"127.0.0.1", "::1"})
	require.Nil(t, err, err)
	tls_cert, err := tls.X509KeyPair(c, k)
	require.Nil(t, err, err)
	tls_config := &tls.Config{
		Certificates: []tls.Certificate{tls_cert},
	}
	listener, err := Listen("tcp", ":0", tls_config)
	require.Nil(t, err, err)
	go func() {
		conn, err := listener.Accept()
		require.Nil(t, err, err)
		write := func() {
			testdata := test_generate_testdata()
			_, err = conn.Write(testdata)
			require.Nil(t, err, err)
			require.Equal(t, test_generate_testdata(), testdata)
		}
		read := func() {
			data := make([]byte, 256)
			_, err = io.ReadFull(conn, data)
			require.Nil(t, err, err)
			require.Equal(t, test_generate_testdata(), data)
		}
		read()
		write()
		write()
		read()
	}()
	// add cert to trust
	tls_config = &tls.Config{
		RootCAs: x509.NewCertPool(),
	}
	x509_cert, err := cert.Parse(c)
	require.Nil(t, err, err)
	tls_config.RootCAs.AddCert(x509_cert)
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.Nil(t, err, err)
	conn, err := Dial("tcp", "localhost:"+port, tls_config)
	require.Nil(t, err, err)
	write := func() {
		testdata := test_generate_testdata()
		_, err = conn.Write(testdata)
		require.Nil(t, err, err)
		require.Equal(t, test_generate_testdata(), testdata)
	}
	read := func() {
		data := make([]byte, 256)
		_, err = io.ReadFull(conn, data)
		require.Nil(t, err, err)
		require.Equal(t, test_generate_testdata(), data)
	}
	write()
	read()
	read()
	write()
}

func test_generate_testdata() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

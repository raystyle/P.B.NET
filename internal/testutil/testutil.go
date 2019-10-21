package testutil

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"sync"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/cert"
)

// PPROF is used to open pprof
func PPROF() {
	go func() {
		_ = http.ListenAndServe("127.0.0.1:1999", nil)
	}()
}

func isDestroyed(object interface{}, gcNum int) bool {
	destroyed := false
	runtime.SetFinalizer(object, func(_ interface{}) {
		destroyed = true
	})
	for i := 0; i < gcNum; i++ {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}
	return destroyed
}

// IsDestroyed is used to check if the object has been recycled by the GC
func IsDestroyed(t require.TestingT, object interface{}, gcNum int) {
	require.True(t, isDestroyed(object, gcNum), "object not destroyed")
}

// TLSConfigPair is used to build server and client tls.Config
func TLSConfigPair(t require.TestingT) (server, client *tls.Config) {
	ca, err := cert.GenerateCA(nil)
	require.NoError(t, err)
	certCfg := &cert.Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	// server
	kp, err := cert.Generate(ca.Certificate, ca.PrivateKey, certCfg)
	require.NoError(t, err)
	tlsCert, err := tls.X509KeyPair(kp.EncodeToPEM())
	require.NoError(t, err)
	server = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	// client
	client = &tls.Config{RootCAs: x509.NewCertPool()}
	client.RootCAs.AddCert(ca.Certificate)
	return
}

// GenerateData is used to generate test data: []byte{0, 1, .... 254, 255}
func GenerateData() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

// Conn is used to client & server Conn Read() Write() and Close()
// if close == true, IsDestroyed will be run after Conn.Close()
//
// if Conn about TLS and use net.Pipe(), set close = false
// server, client := net.Pipe()
// tlsServer = tls.Server(server, cfg)
// Conn(tlsServer, xxx, false) must set false
func Conn(t require.TestingT, server, client net.Conn, close bool) {
	write := func(conn net.Conn) {
		data := GenerateData()
		_, err := conn.Write(data)
		require.NoError(t, err)
		require.Equal(t, GenerateData(), data)
	}
	read := func(conn net.Conn) {
		data := make([]byte, 256)
		_, err := io.ReadFull(conn, data)
		require.NoError(t, err)
		require.Equal(t, GenerateData(), data)
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	// server
	go func() {
		defer wg.Done()
		read(server)
		write(server)
		write(server)
		read(server)
		if close {
			require.NoError(t, server.Close())
			IsDestroyed(t, server, 1)
		}
	}()
	// client
	write(client)
	read(client)
	read(client)
	write(client)
	if close {
		require.NoError(t, client.Close())
		IsDestroyed(t, client, 1)
	}
	wg.Wait()
}

// DeployHTTPSServer is used to deploy a https server for test
// if deploy success, will return server port
func DeployHTTPSServer(t require.TestingT, server *http.Server, kp *cert.KeyPair) string {
	listener, err := net.Listen("tcp", server.Addr)
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(kp.EncodeToPEM())
	require.NoError(t, err)
	server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{tlsCert}}

	// run
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ServeTLS(listener, "", "")
	}()
	select {
	case err = <-errChan:
	case <-time.After(250 * time.Millisecond):
	}
	require.NoError(t, err)

	// get port
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return port
}

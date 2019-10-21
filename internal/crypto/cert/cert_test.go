package cert

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateCA(t *testing.T) {
	ca, err := GenerateCA(nil)
	require.NoError(t, err)
	_, err = tls.X509KeyPair(ca.EncodeToPEM())
	require.NoError(t, err)
}

func TestGenerate(t *testing.T) {
	ca, err := GenerateCA(nil)
	require.NoError(t, err)
	testGenerate(t, ca)  // CA sign
	testGenerate(t, nil) // self sign
}

func testGenerate(t *testing.T, ca *KeyPair) {
	cfg := &Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	var (
		kp  *KeyPair
		err error
	)
	if ca != nil {
		kp, err = Generate(ca.Certificate, ca.PrivateKey, cfg)
		require.NoError(t, err)
	} else {
		kp, err = Generate(nil, nil, cfg)
		require.NoError(t, err)
	}

	server1 := http.Server{Addr: "127.0.0.1:0"}
	port1 := deployHTTPSServer(t, &server1, kp)
	defer func() { _ = server1.Close() }()

	server2 := http.Server{Addr: "[::1]:0"}
	port2 := deployHTTPSServer(t, &server2, kp)
	defer func() { _ = server2.Close() }()

	tlsConfig := tls.Config{RootCAs: x509.NewCertPool()}
	if ca != nil {
		tlsConfig.RootCAs.AddCert(ca.Certificate)
	} else {
		tlsConfig.RootCAs.AddCert(kp.Certificate)
	}
	client := http.Client{Transport: &http.Transport{TLSClientConfig: &tlsConfig}}

	get := func(hostname, port string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:%s/", hostname, port))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "hello", string(b))
	}
	get("localhost", port1)
	get("127.0.0.1", port1)
	get("[::1]", port2)
}

func deployHTTPSServer(t *testing.T, server *http.Server, kp *KeyPair) string {
	listener, err := net.Listen("tcp", server.Addr)
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(kp.EncodeToPEM())
	require.NoError(t, err)
	server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{tlsCert}}

	resp := []byte("hello")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(resp)
	})
	server.Handler = serveMux

	// run
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ServeTLS(listener, "", "")
	}()
	select {
	case err := <-errChan:
		require.NoError(t, err)
	case <-time.After(250 * time.Millisecond):
	}

	// get port
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return port
}

func TestIsDomainName(t *testing.T) {
	require.True(t, isDomainName("asd.com"))
	require.True(t, isDomainName("asd-asd.com"))
	require.True(t, isDomainName("asd-asd6.com"))
	// invalid domain
	require.False(t, isDomainName(""))
	require.False(t, isDomainName(string([]byte{255, 254, 12, 35})))
	require.False(t, isDomainName("asd-"))
	require.False(t, isDomainName("asd.-"))
	require.False(t, isDomainName("asd.."))
	require.False(t, isDomainName(strings.Repeat("a", 64)+".com"))
}

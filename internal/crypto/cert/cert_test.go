package cert

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
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

	// handler
	respData := []byte("hello")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(respData)
	})
	// certificate
	tlsCert, err := kp.TLSCertificate()
	require.NoError(t, err)
	// run https servers
	server1 := http.Server{
		Addr:      "localhost:0",
		Handler:   serveMux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsCert}},
	}
	port1 := testsuite.RunHTTPServer(t, "tcp", &server1)
	defer func() { _ = server1.Close() }()
	// server2
	var (
		server2 http.Server
		port2   string
	)
	if testsuite.EnableIPv4() {
		server2 = http.Server{
			Addr:      "127.0.0.1:0",
			Handler:   serveMux,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsCert}},
		}
		port2 = testsuite.RunHTTPServer(t, "tcp", &server2)
		defer func() { _ = server2.Close() }()
	}
	// server3
	var (
		server3 http.Server
		port3   string
	)
	if testsuite.EnableIPv6() {
		server3 = http.Server{
			Addr:      "[::1]:0",
			Handler:   serveMux,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsCert}},
		}
		port3 = testsuite.RunHTTPServer(t, "tcp", &server3)
		defer func() { _ = server3.Close() }()
	}

	// client
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
		require.Equal(t, respData, b)
	}

	// test
	get("localhost", port1)
	if testsuite.EnableIPv4() {
		get("127.0.0.1", port2)
	}
	if testsuite.EnableIPv6() {
		get("[::1]", port3)
	}
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

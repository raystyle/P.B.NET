package cert

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestGenerateCA(t *testing.T) {
	t.Parallel()
	ca, err := GenerateCA(nil)
	require.NoError(t, err)
	_, err = tls.X509KeyPair(ca.EncodeToPEM())
	require.NoError(t, err)
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	for _, alg := range []string{"rsa", "ecdsa", "ed25519"} {
		t.Run(alg, func(t *testing.T) {
			opts := Options{Algorithm: alg}
			t.Parallel() // must here
			ca, err := GenerateCA(&opts)
			require.NoError(t, err)
			wg := sync.WaitGroup{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				testGenerate(t, ca) // CA sign
			}()
			go func() {
				defer wg.Done()
				testGenerate(t, nil) // self sign
			}()
			wg.Wait()
		})
	}
}

func testGenerate(t *testing.T, ca *KeyPair) {
	opts := &Options{
		Algorithm:   "rsa",
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	var (
		kp  *KeyPair
		err error
	)
	if ca != nil {
		kp, err = Generate(ca.Certificate, ca.PrivateKey, opts)
		require.NoError(t, err)
	} else {
		kp, err = Generate(nil, nil, opts)
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

	// only IPv4
	var port2 string
	if testsuite.EnableIPv4() {
		server2 := http.Server{
			Addr:      "127.0.0.1:0",
			Handler:   serveMux,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsCert}},
		}
		port2 = testsuite.RunHTTPServer(t, "tcp", &server2)
		defer func() { _ = server2.Close() }()
	}

	// only IPv6
	var port3 string
	if testsuite.EnableIPv6() {
		server3 := http.Server{
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

func TestUnknownAlgorithm(t *testing.T) {
	t.Parallel()
	pri, pub, err := genKey("foo alg")
	require.EqualError(t, err, "unknown algorithm: foo alg")
	require.Nil(t, pri)
	require.Nil(t, pub)
	opts := &Options{Algorithm: "foo alg"}
	kp, err := GenerateCA(opts)
	require.Error(t, err)
	require.Nil(t, kp)

	opts.Algorithm = "rsa"
	kp, err = GenerateCA(opts)
	require.NoError(t, err)

	_, err = Generate(kp.Certificate, kp.PrivateKey, nil)
	require.NoError(t, err)

	opts.Algorithm = "foo alg"
	kp, err = Generate(kp.Certificate, kp.PrivateKey, opts)
	require.Error(t, err)
	require.Nil(t, kp)
}

func TestPrint(t *testing.T) {
	t.Parallel()
	ca, err := GenerateCA(nil)
	require.NoError(t, err)
	org := []string{"org a", "org b"}
	ca.Certificate.Subject.Organization = org
	ca.Certificate.Issuer.Organization = org
	t.Log("\n", Print(ca.Certificate))
}

package cert

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
)

func TestGenerateCA(t *testing.T) {
	t.Parallel()
	ca, err := GenerateCA(nil)
	require.NoError(t, err)
	_, err = tls.X509KeyPair(ca.EncodeToPEM())
	require.NoError(t, err)

	// set options
	now := time.Now()
	notAfter := now.AddDate(0, 0, 1)
	opts := &Options{
		Algorithm: "rsa|1024",
		NotBefore: now,
		NotAfter:  notAfter,
	}
	opts.Subject.CommonName = "test common name"
	opts.Subject.Organization = []string{"test organization"}
	ca, err = GenerateCA(opts)
	require.NoError(t, err)
	require.Equal(t, "test common name", ca.Certificate.Subject.CommonName)
	require.Equal(t, []string{"test organization"}, ca.Certificate.Subject.Organization)
	excepted := now.Format(logger.TimeLayout)
	actual := ca.Certificate.NotBefore.Local().Format(logger.TimeLayout)
	require.Equal(t, excepted, actual)
	excepted = notAfter.Format(logger.TimeLayout)
	actual = ca.Certificate.NotAfter.Local().Format(logger.TimeLayout)
	require.Equal(t, excepted, actual)

	// invalid domain name
	opts.DNSNames = []string{"foo-"}
	_, err = GenerateCA(opts)
	require.Error(t, err)

	opts.DNSNames = nil
	// invalid IP address
	opts.IPAddresses = []string{"foo ip"}
	_, err = GenerateCA(opts)
	require.Error(t, err)
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	for _, alg := range []string{"", "rsa|1024", "ecdsa|p224", "ed25519"} {
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

	// invalid domain name
	opts := new(Options)
	opts.DNSNames = []string{"foo-"}
	_, err := Generate(nil, nil, opts)
	require.Error(t, err)

	opts.DNSNames = nil
	// create failed
	_, err = Generate(new(x509.Certificate), "foo", opts)
	require.Error(t, err)
}

func testGenerate(t *testing.T, ca *Pair) {
	opts := &Options{
		Algorithm:   "rsa|1024",
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	var (
		pair *Pair
		err  error
	)
	if ca != nil {
		pair, err = Generate(ca.Certificate, ca.PrivateKey, opts)
		require.NoError(t, err)
	} else {
		pair, err = Generate(nil, nil, opts)
		require.NoError(t, err)
	}
	// handler
	respData := []byte("hello")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(respData)
	})
	// certificate
	tlsCert, err := pair.TLSCertificate()
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
		tlsConfig.RootCAs.AddCert(pair.Certificate)
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

func TestGeneratePrivateKey(t *testing.T) {
	t.Parallel()
	// rsa with invalid bits
	_, _, err := generatePrivateKey("rsa|foo")
	require.Error(t, err)
	// ecdsa
	_, _, err = generatePrivateKey("ecdsa|p256")
	require.NoError(t, err)
	_, _, err = generatePrivateKey("ecdsa|p384")
	require.NoError(t, err)
	_, _, err = generatePrivateKey("ecdsa|p521")
	require.NoError(t, err)
	_, _, err = generatePrivateKey("ecdsa|foo")
	require.Error(t, err)
}

func TestUnknownAlgorithm(t *testing.T) {
	t.Parallel()
	pri, pub, err := generatePrivateKey("foo|alg")
	require.EqualError(t, err, "unknown algorithm: foo|alg")
	require.Nil(t, pri)
	require.Nil(t, pub)
	opts := &Options{Algorithm: "foo alg"}
	pair, err := GenerateCA(opts)
	require.Error(t, err)
	require.Nil(t, pair)

	opts.Algorithm = "rsa|1024"
	pair, err = GenerateCA(opts)
	require.NoError(t, err)

	_, err = Generate(pair.Certificate, pair.PrivateKey, nil)
	require.NoError(t, err)

	opts.Algorithm = "foo|alg"
	pair, err = Generate(pair.Certificate, pair.PrivateKey, opts)
	require.Error(t, err)
	require.Nil(t, pair)
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

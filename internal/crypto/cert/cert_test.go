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

func TestIsDomainName(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		testdata := []string{
			"test.com",
			"Test-sub.com",
			"test-sub2.com",
		}
		for i := 0; i < len(testdata); i++ {
			require.True(t, isDomainName(testdata[i]))
		}
	})

	t.Run("invalid", func(t *testing.T) {
		testdata := []string{
			"",
			string([]byte{255, 254, 12, 35}),
			"test-",
			"Test.-",
			"test..",
			strings.Repeat("a", 64) + ".com",
		}
		for i := 0; i < len(testdata); i++ {
			require.False(t, isDomainName(testdata[i]))
		}
	})
}

func TestGenerateCA(t *testing.T) {
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
	excepted := now.Format(timeLayout)
	actual := ca.Certificate.NotBefore.Local().Format(timeLayout)
	require.Equal(t, excepted, actual)
	excepted = notAfter.Format(timeLayout)
	actual = ca.Certificate.NotAfter.Local().Format(timeLayout)
	require.Equal(t, excepted, actual)

	t.Run("invalid domain name", func(t *testing.T) {
		opts.DNSNames = []string{"foo-"}
		_, err = GenerateCA(opts)
		require.Error(t, err)
		opts.DNSNames = nil
	})

	t.Run("invalid IP address", func(t *testing.T) {
		opts.IPAddresses = []string{"foo ip"}
		_, err = GenerateCA(opts)
		require.Error(t, err)
	})
}

func TestGenerate(t *testing.T) {
	for _, alg := range []string{"", "rsa|1024", "ecdsa|p224", "ed25519"} {
		t.Run(alg, func(t *testing.T) {
			opts := Options{Algorithm: alg}
			ca, err := GenerateCA(&opts)
			require.NoError(t, err)
			testGenerate(t, ca)  // CA sign
			testGenerate(t, nil) // self sign
		})
	}

	t.Run("invalid domain name", func(t *testing.T) {
		opts := Options{
			DNSNames: []string{"foo-"},
		}
		_, err := Generate(nil, nil, &opts)
		require.Error(t, err)
	})

	t.Run("invalid private key", func(t *testing.T) {
		_, err := Generate(new(x509.Certificate), "foo", nil)
		require.Error(t, err)
	})
}

func testRunHTTPServer(t testing.TB, network string, server *http.Server) string {
	listener, err := net.Listen(network, server.Addr)
	require.NoError(t, err)
	// run
	go func() {
		if server.TLSConfig != nil {
			_ = server.ServeTLS(listener, "", "")
		} else {
			_ = server.Serve(listener)
		}
	}()
	// get port
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return port
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
	require.Equal(t, pair.Certificate.Raw, pair.ASN1())

	// handler
	respData := []byte("hello")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(respData)
	})

	tlsCert := pair.TLSCertificate()
	// run https servers
	server1 := http.Server{
		Addr:      "localhost:0",
		Handler:   serveMux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsCert}},
	}
	port1 := testRunHTTPServer(t, "tcp", &server1)
	defer func() { _ = server1.Close() }()
	// IPv4-only
	server2 := http.Server{
		Addr:      "127.0.0.1:0",
		Handler:   serveMux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsCert}},
	}
	port2 := testRunHTTPServer(t, "tcp", &server2)
	defer func() { _ = server2.Close() }()
	// IPv6-only
	server3 := http.Server{
		Addr:      "[::1]:0",
		Handler:   serveMux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsCert}},
	}
	port3 := testRunHTTPServer(t, "tcp", &server3)
	defer func() { _ = server3.Close() }()

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
	get("127.0.0.1", port2)
	get("[::1]", port3)
}

func TestGeneratePrivateKey(t *testing.T) {
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
	ca, err := GenerateCA(nil)
	require.NoError(t, err)
	org := []string{"org a", "org b"}
	ca.Certificate.Subject.Organization = org
	ca.Certificate.Issuer.Organization = org
	t.Log("\n", Print(ca.Certificate))
}

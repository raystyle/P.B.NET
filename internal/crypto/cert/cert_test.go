package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/rsa"
)

func TestGenerateCA(t *testing.T) {
	caCert, caKey := GenerateCA(&Config{})
	_, err := Parse(caCert)
	require.NoError(t, err)
	_, err = rsa.ImportPrivateKeyPEM(caKey)
	require.NoError(t, err)
	// invalid pem
	_, err = Parse(nil)
	require.Equal(t, err, ErrInvalidPEMBlock)
	// invalid type
	block := pem.Block{}
	block.Type = "CERTIFICATE asdsad"
	_, err = Parse(pem.EncodeToMemory(&block))
	require.Equal(t, err, ErrInvalidPEMBlockType)
}

func TestGenerate(t *testing.T) {
	caCert, caKey := GenerateCA(new(Config))
	parent, err := Parse(caCert)
	require.NoError(t, err)
	caPri, err := rsa.ImportPrivateKeyPEM(caKey)
	require.NoError(t, err)
	c := &Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	cert, key, err := Generate(parent, caPri, c)
	require.NoError(t, err)
	s := http.Server{}
	port := mockHTTPSServer(t, &s, cert, key)
	defer func() { _ = s.Close() }()
	tlsConfig := tls.Config{RootCAs: x509.NewCertPool()}
	tlsConfig.RootCAs.AddCert(parent)
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}
	get := func(hostname string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:%s/", hostname, port))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		t.Log(string(b))
	}
	get("localhost")
	get("127.0.0.1")
	// get("[::1]")
}

func TestGenerate_Self(t *testing.T) {
	c := &Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	cert, key, err := Generate(nil, nil, c)
	require.NoError(t, err)
	s := http.Server{}
	port := mockHTTPSServer(t, &s, cert, key)
	defer func() { _ = s.Close() }()
	// not add trust and check error
	client := http.Client{}
	_, err = client.Get("https://localhost:" + port + "/")
	require.Error(t, err)
	// add trust
	cer, err := Parse(cert)
	require.NoError(t, err)
	tlsConfig := tls.Config{RootCAs: x509.NewCertPool()}
	tlsConfig.RootCAs.AddCert(cer)
	client = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}
	get := func(hostname string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:%s/", hostname, port))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		t.Log(string(b))
	}
	get("localhost")
	get("127.0.0.1")
	// get("[::1]")
}

func mockHTTPSServer(t *testing.T, s *http.Server, cert, key []byte) string {
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	certificate, err := tls.X509KeyPair(cert, key)
	require.NoError(t, err)
	s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{certificate}}
	data := []byte("hello")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	s.Handler = serveMux
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.ServeTLS(l, "", "")
		close(errChan)
	}()
	// start
	select {
	case err := <-errChan:
		t.Fatal(err)
	case <-time.After(250 * time.Millisecond):
	}
	// get port
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	return port
}

func TestInvalid(t *testing.T) {
	caCert, caKey := GenerateCA(new(Config))
	parent, err := Parse(caCert)
	require.NoError(t, err)
	privatekey, err := rsa.ImportPrivateKeyPEM(caKey)
	require.NoError(t, err)
	// invalid privatekey
	privatekey.PublicKey.N.SetBytes(nil)
	_, _, err = Generate(parent, privatekey, new(Config))
	require.Error(t, err)
}

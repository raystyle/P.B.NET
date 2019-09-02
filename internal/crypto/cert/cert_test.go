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

func Test_Generate_CA(t *testing.T) {
	ca_cert, ca_key := Generate_CA(&Config{})
	_, err := Parse(ca_cert)
	require.NoError(t, err)
	_, err = rsa.Import_PrivateKey_PEM(ca_key)
	require.NoError(t, err)
	// invalid pem
	_, err = Parse(nil)
	require.Equal(t, err, ERR_INVALID_PEM_BLOCK, err)
	// invalid type
	block := &pem.Block{}
	block.Type = "CERTIFICATE asdsad"
	_, err = Parse(pem.EncodeToMemory(block))
	require.Equal(t, err, ERR_INVALID_PEM_BLOCK_TYPE, err)
}

func Test_Generate(t *testing.T) {
	ca_cert, ca_key := Generate_CA(new(Config))
	parent, err := Parse(ca_cert)
	require.NoError(t, err)
	ca_pri, err := rsa.Import_PrivateKey_PEM(ca_key)
	require.NoError(t, err)
	c := &Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	cert, key, err := Generate(parent, ca_pri, c)
	require.NoError(t, err)
	s := &http.Server{}
	port := mock_https_server(t, s, cert, key)
	defer func() { _ = s.Close() }()
	tls_config := &tls.Config{RootCAs: x509.NewCertPool()}
	tls_config.RootCAs.AddCert(parent)
	client := http.Client{Transport: &http.Transport{TLSClientConfig: tls_config}}
	get := func(hostname string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:%s/", hostname, port))
		require.NoError(t, err)
		defer func() {
			_ = resp.Body.Close()
		}()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		t.Log(string(b))
	}
	get("localhost")
	get("127.0.0.1")
	get("[::1]")
}

func Test_Generate_Self(t *testing.T) {
	c := &Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	cert, key, err := Generate(nil, nil, c)
	require.NoError(t, err)
	s := &http.Server{}
	port := mock_https_server(t, s, cert, key)
	defer func() { _ = s.Close() }()
	// not add trust and check error
	client := http.Client{}
	_, err = client.Get("https://localhost:" + port + "/")
	require.NotNil(t, err)
	// add trust
	cer, err := Parse(cert)
	require.NoError(t, err)
	tls_config := &tls.Config{RootCAs: x509.NewCertPool()}
	tls_config.RootCAs.AddCert(cer)
	client = http.Client{Transport: &http.Transport{TLSClientConfig: tls_config}}
	get := func(hostname string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:%s/", hostname, port))
		require.NoError(t, err)
		defer func() {
			_ = resp.Body.Close()
		}()
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		t.Log(string(b))
	}
	get("localhost")
	get("127.0.0.1")
	get("[::1]")
}

func Test_Invalid(t *testing.T) {
	ca_cert, ca_key := Generate_CA(new(Config))
	parent, err := Parse(ca_cert)
	require.NoError(t, err)
	privatekey, err := rsa.Import_PrivateKey_PEM(ca_key)
	require.NoError(t, err)
	// invalid privatekey
	privatekey.PublicKey.N.SetBytes(nil)
	_, _, err = Generate(parent, privatekey, new(Config))
	require.NotNil(t, err)
}

func mock_https_server(t *testing.T, s *http.Server, cert, key []byte) string {
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	certificate, err := tls.X509KeyPair(cert, key)
	require.NoError(t, err)
	s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{certificate}}
	data := []byte("hello")
	server_mux := http.NewServeMux()
	server_mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	s.Handler = server_mux
	err_chan := make(chan error, 1)
	go func() {
		err_chan <- s.ServeTLS(l, "", "")
		close(err_chan)
	}()
	// start
	select {
	case err := <-err_chan:
		t.Fatal(err)
	case <-time.After(250 * time.Millisecond):
	}
	// get port
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	return port
}

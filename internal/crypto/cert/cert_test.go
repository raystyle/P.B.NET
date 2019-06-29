package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/rsa"
)

func Test_Generate_CA(t *testing.T) {
	ca_cert, ca_key := Generate_CA(&Config{})
	_, err := Parse(ca_cert)
	require.Nil(t, err, err)
	_, err = rsa.Import_PrivateKey_PEM(ca_key)
	require.Nil(t, err, err)
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
	require.Nil(t, err, err)
	ca_pri, err := rsa.Import_PrivateKey_PEM(ca_key)
	require.Nil(t, err, err)
	c := &Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	cert, key, err := Generate(parent, ca_pri, c)
	require.Nil(t, err, err)
	wg := new(sync.WaitGroup)
	port, stop_signal := mock_https_server(t, wg, cert, key)
	defer func() {
		stop_signal <- struct{}{}
		wg.Wait()
	}()
	tls_config := &tls.Config{RootCAs: x509.NewCertPool()}
	tls_config.RootCAs.AddCert(parent)
	client := http.Client{Transport: &http.Transport{TLSClientConfig: tls_config}}
	get := func(hostname string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:%s/", hostname, port))
		require.Nil(t, err, err)
		defer func() {
			_ = resp.Body.Close()
		}()
		b, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err, err)
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
	require.Nil(t, err, err)
	wg := &sync.WaitGroup{}
	port, stop_signal := mock_https_server(t, wg, cert, key)
	defer func() {
		stop_signal <- struct{}{}
		wg.Wait()
	}()
	// not add trust and check error
	client := http.Client{}
	_, err = client.Get("https://localhost:" + port + "/")
	require.NotNil(t, err)
	// add trust
	cer, err := Parse(cert)
	require.Nil(t, err, err)
	tls_config := &tls.Config{RootCAs: x509.NewCertPool()}
	tls_config.RootCAs.AddCert(cer)
	client = http.Client{Transport: &http.Transport{TLSClientConfig: tls_config}}
	get := func(hostname string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:%s/", hostname, port))
		require.Nil(t, err, err)
		defer func() {
			_ = resp.Body.Close()
		}()
		b, err := ioutil.ReadAll(resp.Body)
		require.Nil(t, err, err)
		t.Log(string(b))
	}
	get("localhost")
	get("127.0.0.1")
	get("[::1]")
}

func Test_Invalid(t *testing.T) {
	ca_cert, ca_key := Generate_CA(new(Config))
	parent, err := Parse(ca_cert)
	require.Nil(t, err, err)
	privatekey, err := rsa.Import_PrivateKey_PEM(ca_key)
	require.Nil(t, err, err)
	// invalid privatekey
	privatekey.PublicKey.N.SetBytes(nil)
	_, _, err = Generate(parent, privatekey, new(Config))
	require.NotNil(t, err)
}

func mock_https_server(t *testing.T, wg *sync.WaitGroup, cert, key []byte) (string, chan struct{}) {
	l, err := net.Listen("tcp", ":0")
	require.Nil(t, err, err)
	// tls and http2.0
	certificate, err := tls.X509KeyPair(cert, key)
	require.Nil(t, err, err)
	server := http.Server{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
		},
	}
	data := []byte("hello")
	server_mux := http.NewServeMux()
	server_mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	server.Handler = server_mux
	err_chan := make(chan error, 1)
	wg.Add(1)
	go func() {
		err_chan <- server.ServeTLS(l, "", "")
		close(err_chan)
		wg.Done()
	}()
	// start
	select {
	case err := <-err_chan:
		t.Fatal(err)
	case <-time.After(time.Millisecond * 500):
	}
	// stop
	stop_signal := make(chan struct{}, 1)
	wg.Add(1)
	go func() {
		<-stop_signal
		_ = server.Close()
		wg.Done()
	}()
	// get port
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.Nil(t, err, err)
	return port, stop_signal
}

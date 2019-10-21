package bootstrap

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/ed25519"
)

func TestHTTP(t *testing.T) {
	// init mock proxy pool
	proxyPool := newMockProxyPool(t)
	defer proxyPool.Close()
	// generate bootstrap nodes info
	nodes := testGenerateNodes()
	// --------------------------http---------------------------
	HTTP := testGenerateHTTP(t, proxyPool)
	info := HTTP.Generate(nodes)
	t.Log("(http) bootstrap nodes info:", info)
	// init mock http server
	httpServer := http.Server{
		Addr: "localhost:0",
	}
	port := testStartHTTPServer(t, &httpServer, info)
	defer func() { _ = httpServer.Close() }()
	// config
	HTTP.Request.URL = "http://localhost:" + port
	// marshal
	b, err := HTTP.Marshal()
	require.NoError(t, err)
	// unmarshal
	HTTP = NewHTTP(proxyPool, new(mockDNSClient))
	err = HTTP.Unmarshal(b)
	require.NoError(t, err)
	resolved, err := HTTP.Resolve()
	require.NoError(t, err)
	require.Equal(t, nodes, resolved)
	// --------------------------https--------------------------
	HTTP = testGenerateHTTP(t, proxyPool)
	info = HTTP.Generate(nodes)
	require.NoError(t, err)
	t.Log("(https) bootstrap nodes info:", info)
	// init mock https server
	certCfg := &cert.Config{
		DNSNames: []string{"localhost"},
	}
	c, k, err := cert.Generate(nil, nil, certCfg)
	require.NoError(t, err)
	tlsCert, err := tls.X509KeyPair(c, k)
	require.NoError(t, err)
	httpsServer := http.Server{
		Addr: "localhost:0",
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		}}
	port = testStartHTTPServer(t, &httpsServer, info)
	defer func() { _ = httpsServer.Close() }()
	// config
	HTTP.Request.URL = "https://localhost:" + port
	// add cert to trust
	HTTP.Transport.TLSClientConfig.RootCAs = []string{string(c)}
	// marshal
	b, err = HTTP.Marshal()
	require.NoError(t, err)
	// unmarshal
	HTTP = NewHTTP(proxyPool, new(mockDNSClient))
	err = HTTP.Unmarshal(b)
	require.NoError(t, err)
	resolved, err = HTTP.Resolve()
	require.NoError(t, err)
	require.Equal(t, nodes, resolved)
}

func testGenerateHTTP(t *testing.T, p *mockProxyPool) *HTTP {
	HTTP := NewHTTP(p, new(mockDNSClient))
	HTTP.AESKey = strings.Repeat("FF", aes.Bit256)
	HTTP.AESIV = strings.Repeat("FF", aes.IVSize)
	// generate privateKey
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.PrivateKey = privateKey
	return HTTP
}

// return port
func testStartHTTPServer(t *testing.T, s *http.Server, info string) string {
	l, err := net.Listen("tcp", s.Addr)
	require.NoError(t, err)
	data := []byte(info)
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	s.Handler = serveMux
	errChan := make(chan error, 1)
	go func() {
		if s.TLSConfig == nil {
			errChan <- s.Serve(l)
		} else {
			errChan <- s.ServeTLS(l, "", "")
		}
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

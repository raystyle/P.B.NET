package bootstrap

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/ed25519"
	"project/internal/testutil"
)

func testGenerateHTTP(t *testing.T, p *mockProxyPool) *HTTP {
	HTTP := NewHTTP(p, new(mockDNSClient))
	HTTP.AESKey = strings.Repeat("FF", aes.Key256Bit)
	HTTP.AESIV = strings.Repeat("FF", aes.IVSize)
	// generate privateKey
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.PrivateKey = privateKey
	return HTTP
}

func TestHTTP(t *testing.T) {
	// init mock proxy pool
	proxyPool := newMockProxyPool(t)
	defer proxyPool.Close()
	dnsClient := new(mockDNSClient)
	// generate bootstrap nodes info
	nodes := testGenerateNodes()
	// --------------------------http---------------------------
	HTTP := testGenerateHTTP(t, proxyPool)
	nodesInfo := HTTP.Generate(nodes)
	t.Log("(http) bootstrap nodes info:", nodesInfo)

	// deploy test http server
	serveMux := http.NewServeMux()
	nodesData := []byte(nodesInfo)
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(nodesData)
	})
	httpServer := http.Server{
		Addr:    "localhost:0",
		Handler: serveMux,
	}
	port := testutil.DeployHTTPServer(t, &httpServer, nil)
	defer func() { _ = httpServer.Close() }()
	// config
	HTTP.Request.URL = "http://localhost:" + port
	// marshal
	b, err := HTTP.Marshal()
	require.NoError(t, err)
	// unmarshal
	HTTP = NewHTTP(proxyPool, dnsClient)
	err = HTTP.Unmarshal(b)
	require.NoError(t, err)
	resolved, err := HTTP.Resolve()
	require.NoError(t, err)
	require.Equal(t, nodes, resolved)
	// --------------------------https--------------------------
	HTTP = testGenerateHTTP(t, proxyPool)
	nodesInfo = HTTP.Generate(nodes)
	require.NoError(t, err)
	t.Log("(https) bootstrap nodes info:", nodesInfo)
	nodesData = []byte(nodesInfo)

	// deploy test https server
	certCfg := cert.Config{
		DNSNames:    []string{"localhost"},
		IPAddresses: []string{"127.0.0.1", "::1"},
	}
	kp, err := cert.Generate(nil, nil, &certCfg)
	require.NoError(t, err)
	httpsServer := http.Server{
		Addr:    "localhost:0",
		Handler: serveMux,
	}
	port = testutil.DeployHTTPServer(t, &httpServer, kp)
	defer func() { _ = httpsServer.Close() }()
	// config
	HTTP.Request.URL = "https://localhost:" + port
	// add cert to trust
	certPEM, _ := kp.EncodeToPEM()
	HTTP.Transport.TLSClientConfig.RootCAs = []string{string(certPEM)}
	// marshal
	b, err = HTTP.Marshal()
	require.NoError(t, err)
	// unmarshal
	HTTP = NewHTTP(proxyPool, dnsClient)
	err = HTTP.Unmarshal(b)
	require.NoError(t, err)
	resolved, err = HTTP.Resolve()
	require.NoError(t, err)
	require.Equal(t, nodes, resolved)
}

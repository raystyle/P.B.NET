package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
)

func testGenerateHTTP(t *testing.T, p *proxy.Pool, c *dns.Client) *HTTP {
	HTTP := NewHTTP(context.Background(), p, c)
	HTTP.AESKey = strings.Repeat("FF", aes.Key256Bit)
	HTTP.AESIV = strings.Repeat("FF", aes.IVSize)
	// generate privateKey
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	HTTP.PrivateKey = privateKey
	return HTTP
}

func TestHTTP(t *testing.T) {
	client, pool, manager := testdns.DNSClient(t)
	defer func() { _ = manager.Close() }()
	// generate bootstrap nodes info
	nodes := testGenerateNodes()

	// --------------------------http---------------------------
	HTTP := testGenerateHTTP(t, pool, client)
	nodesInfo := HTTP.Generate(nodes)
	t.Log("(http) bootstrap nodes info:", nodesInfo)

	// set test http server mux
	serveMux := http.NewServeMux()
	nodesData := []byte(nodesInfo)
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(nodesData)
	})

	if testsuite.EnableIPv4() {
		httpServer := http.Server{
			Addr:    "localhost:0",
			Handler: serveMux,
		}
		port := testsuite.RunHTTPServer(t, "tcp4", &httpServer)
		defer func() { _ = httpServer.Close() }()

		// config
		HTTP.Request.URL = "http://localhost:" + port
		HTTP.DNSOpts.Mode = dns.ModeSystem
		HTTP.DNSOpts.Type = dns.TypeIPv4
		// marshal
		b, err := HTTP.Marshal()
		require.NoError(t, err)
		// unmarshal
		HTTP = NewHTTP(context.Background(), pool, client)
		err = HTTP.Unmarshal(b)
		require.NoError(t, err)
		resolved, err := HTTP.Resolve()
		require.NoError(t, err)
		require.Equal(t, nodes, resolved)
	}

	if testsuite.EnableIPv6() {
		httpServer := http.Server{
			Addr:    "localhost:0",
			Handler: serveMux,
		}
		port := testsuite.RunHTTPServer(t, "tcp6", &httpServer)
		defer func() { _ = httpServer.Close() }()

		// config
		HTTP.Request.URL = "http://localhost:" + port
		HTTP.DNSOpts.Mode = dns.ModeSystem
		HTTP.DNSOpts.Type = dns.TypeIPv6
		// marshal
		b, err := HTTP.Marshal()
		require.NoError(t, err)
		// unmarshal
		HTTP = NewHTTP(context.Background(), pool, client)
		err = HTTP.Unmarshal(b)
		require.NoError(t, err)
		resolved, err := HTTP.Resolve()
		require.NoError(t, err)
		require.Equal(t, nodes, resolved)
	}

	// --------------------------https--------------------------
	HTTP = testGenerateHTTP(t, pool, client)
	nodesInfo = HTTP.Generate(nodes)
	t.Log("(https) bootstrap nodes info:", nodesInfo)
	nodesData = []byte(nodesInfo)
	serverCfg, clientCfg := testsuite.TLSConfigOptionPair(t)

	if testsuite.EnableIPv4() {
		tlsConfig, err := serverCfg.Apply()
		require.NoError(t, err)
		httpsServer := http.Server{
			Addr:      "localhost:0",
			Handler:   serveMux,
			TLSConfig: tlsConfig,
		}
		port := testsuite.RunHTTPServer(t, "tcp4", &httpsServer)
		defer func() { _ = httpsServer.Close() }()
		// config
		HTTP.Request.URL = "https://localhost:" + port
		HTTP.DNSOpts.Mode = dns.ModeSystem
		HTTP.DNSOpts.Type = dns.TypeIPv4
		HTTP.Transport.TLSClientConfig = *clientCfg
		// marshal
		b, err := HTTP.Marshal()
		require.NoError(t, err)
		// unmarshal
		HTTP = NewHTTP(context.Background(), pool, client)
		err = HTTP.Unmarshal(b)
		require.NoError(t, err)
		resolved, err := HTTP.Resolve()
		require.NoError(t, err)
		require.Equal(t, nodes, resolved)
	}

	if testsuite.EnableIPv6() {
		tlsConfig, err := serverCfg.Apply()
		require.NoError(t, err)
		httpsServer := http.Server{
			Addr:      "localhost:0",
			Handler:   serveMux,
			TLSConfig: tlsConfig,
		}
		port := testsuite.RunHTTPServer(t, "tcp6", &httpsServer)
		defer func() { _ = httpsServer.Close() }()
		// config
		HTTP.Request.URL = "https://localhost:" + port
		HTTP.DNSOpts.Mode = dns.ModeSystem
		HTTP.DNSOpts.Type = dns.TypeIPv6
		HTTP.Transport.TLSClientConfig = *clientCfg
		// marshal
		b, err := HTTP.Marshal()
		require.NoError(t, err)
		// unmarshal
		HTTP = NewHTTP(context.Background(), pool, client)
		err = HTTP.Unmarshal(b)
		require.NoError(t, err)
		resolved, err := HTTP.Resolve()
		require.NoError(t, err)
		require.Equal(t, nodes, resolved)
	}

	testsuite.IsDestroyed(t, HTTP)
}

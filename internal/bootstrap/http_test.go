package bootstrap

import (
	"bytes"
	"context"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
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
	nodesInfo, err := HTTP.Generate(nodes)
	require.NoError(t, err)
	t.Logf("(http) bootstrap nodes info: %s\n", nodesInfo)

	// set test http server mux
	serveMux := http.NewServeMux()
	nodesData := nodesInfo
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(nodesData)
	})

	if testsuite.EnableIPv4() {
		// run HTTP server
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

		for i := 0; i < 10; i++ {
			resolved, err := HTTP.Resolve()
			require.NoError(t, err)
			require.Equal(t, nodes, resolved)
		}
	}

	if testsuite.EnableIPv6() {
		// run HTTP server
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

		for i := 0; i < 10; i++ {
			resolved, err := HTTP.Resolve()
			require.NoError(t, err)
			require.Equal(t, nodes, resolved)
		}
	}

	// --------------------------https--------------------------
	HTTP = testGenerateHTTP(t, pool, client)
	nodesInfo, err = HTTP.Generate(nodes)
	require.NoError(t, err)
	t.Logf("(https) bootstrap nodes info: %s\n", nodesInfo)
	nodesData = nodesInfo
	serverCfg, clientCfg := testsuite.TLSConfigOptionPair(t)

	if testsuite.EnableIPv4() {
		// run HTTPS server
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

		for i := 0; i < 10; i++ {
			resolved, err := HTTP.Resolve()
			require.NoError(t, err)
			require.Equal(t, nodes, resolved)
		}
	}

	if testsuite.EnableIPv6() {
		// run HTTPS server
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

		for i := 0; i < 10; i++ {
			resolved, err := HTTP.Resolve()
			require.NoError(t, err)
			require.Equal(t, nodes, resolved)
		}
	}

	testsuite.IsDestroyed(t, HTTP)
}

func TestHTTP_Validate(t *testing.T) {
	HTTP := NewHTTP(nil, nil, nil)
	// invalid request
	require.Error(t, HTTP.Validate())

	// invalid transport
	HTTP.Request.URL = "http://abc.com/"
	HTTP.Transport.TLSClientConfig.RootCAs = []string{"foo ca"}
	require.Error(t, HTTP.Validate())

	HTTP.Transport.TLSClientConfig.RootCAs = nil

	// invalid AES Key
	HTTP.AESKey = "foo key"
	require.Error(t, HTTP.Validate())

	HTTP.AESKey = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.Key128Bit))

	// invalid AES IV
	HTTP.AESIV = "foo iv"

	b, err := HTTP.Marshal()
	require.Error(t, err)
	require.Nil(t, b)
}

func TestHTTP_Generate(t *testing.T) {
	HTTP := NewHTTP(nil, nil, nil)

	// no bootstrap nodes
	_, err := HTTP.Generate(nil)
	require.Error(t, err)

	// invalid AES Key
	HTTP.PrivateKey, err = ed25519.GenerateKey()
	require.NoError(t, err)
	nodes := testGenerateNodes()
	HTTP.AESKey = "foo key"
	_, err = HTTP.Generate(nodes)
	require.Error(t, err)

	HTTP.AESKey = hex.EncodeToString(bytes.Repeat([]byte{0}, aes.Key128Bit))

	// invalid AES IV
	HTTP.AESIV = "foo iv"
	_, err = HTTP.Generate(nodes)
	require.Error(t, err)

	// invalid Key IV
	HTTP.AESIV = hex.EncodeToString(bytes.Repeat([]byte{0}, 32))
	_, err = HTTP.Generate(nodes)
	require.Error(t, err)
}

func TestHTTP_Unmarshal(t *testing.T) {
	HTTP := NewHTTP(nil, nil, nil)

	// unmarshal invalid config
	require.Error(t, HTTP.Unmarshal([]byte{0x00}))

	// with incorrect config
	require.Error(t, HTTP.Unmarshal(nil))
}

func TestHTTP_Resolve(t *testing.T) {

}

func TestHTTPPanic(t *testing.T) {

}

func TestHTTPOptions(t *testing.T) {
	config, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	HTTP := NewHTTP(nil, nil, nil)
	require.NoError(t, toml.Unmarshal(config, HTTP))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: 15 * time.Second, actual: HTTP.Timeout},
		{expected: "balance", actual: HTTP.ProxyTag},
		{expected: int64(65535), actual: HTTP.MaxBodySize},
		{expected: "FF", actual: HTTP.AESKey},
		{expected: "AA", actual: HTTP.AESIV},
		{expected: "E3", actual: HTTP.PublicKey},
		{expected: "http://abc.com/", actual: HTTP.Request.URL},
		{expected: 2, actual: HTTP.Transport.MaxIdleConns},
		{expected: dns.ModeSystem, actual: HTTP.DNSOpts.Mode},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

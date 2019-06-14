package bootstrap

import (
	"crypto/elliptic"
	"crypto/tls"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/ecdsa"
	"project/internal/dns"
)

func Test_HTTP(t *testing.T) {
	// init mock proxy pool
	proxy_pool := new(mock_proxy_pool)
	proxy_pool.Init(t)
	defer proxy_pool.Close()
	// generate bootstrap nodes info
	nodes := test_generate_nodes()
	//--------------------------http---------------------------
	HTTP := test_generate_http(t, proxy_pool)
	info, err := HTTP.Generate(nodes)
	require.Nil(t, err, err)
	t.Log("(http) bootstrap nodes info:", info)
	// init mock http server
	s := new(http.Server)
	port := test_start_http_server(t, s, info)
	defer func() { _ = s.Close() }()
	// config
	HTTP.Request.URL = "http://" + test_domain + ":" + port
	// marshal
	b, err := HTTP.Marshal()
	require.Nil(t, err, err)
	// unmarshal
	HTTP = New_HTTP(new(mock_resolver), proxy_pool)
	err = HTTP.Unmarshal(b)
	require.Nil(t, err, err)
	resolved, err := HTTP.Resolve()
	require.Nil(t, err, err)
	require.Equal(t, nodes, resolved)
	//--------------------------https--------------------------
	HTTP = test_generate_http(t, proxy_pool)
	info, err = HTTP.Generate(nodes)
	require.Nil(t, err, err)
	t.Log("(https) bootstrap nodes info:", info)
	// init mock https server
	c, k, err := cert.Generate(nil, nil,
		[]string{"localhost"}, []string{"127.0.0.1", "::1"})
	require.Nil(t, err, err)
	tls_cert, err := tls.X509KeyPair(c, k)
	require.Nil(t, err, err)
	s = &http.Server{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tls_cert},
		}}
	port = test_start_http_server(t, s, info)
	defer func() { _ = s.Close() }()
	// config
	HTTP.Request.URL = "https://" + test_domain + ":" + port
	// add cert to trust
	HTTP.Transport.TLSClientConfig.RootCAs = []string{string(c)}
	// set ipv6
	HTTP.DNS_Opts.Type = dns.IPV6
	// marshal
	b, err = HTTP.Marshal()
	require.Nil(t, err, err)
	// unmarshal
	HTTP = New_HTTP(new(mock_resolver), proxy_pool)
	err = HTTP.Unmarshal(b)
	require.Nil(t, err, err)
	resolved, err = HTTP.Resolve()
	require.Nil(t, err, err)
	require.Equal(t, nodes, resolved)
}

func test_generate_http(t *testing.T, p *mock_proxy_pool) *HTTP {
	HTTP := New_HTTP(new(mock_resolver), p)
	HTTP.AES_Key = strings.Repeat("FF", aes.BIT256)
	HTTP.AES_IV = strings.Repeat("FF", aes.IV_SIZE)
	// generate privatekey
	privatekey, err := ecdsa.Generate_Key(elliptic.P256())
	require.Nil(t, err, err)
	HTTP.PrivateKey = privatekey
	return HTTP
}

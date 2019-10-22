package bootstrap

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/http"
	"project/internal/proxy/socks5"
	"project/internal/xnet"
)

func testGenerateNodes() []*Node {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:53123",
	}
	nodes[1] = &Node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "[::1]:53123",
	}
	return nodes
}

type mockDNSClient struct{}

func (dr *mockDNSClient) Resolve(_ string, opts *dns.Options) ([]string, error) {
	if opts == nil {
		opts = new(dns.Options)
	}
	switch opts.Type {
	case "", dns.IPv4:
		return []string{"127.0.0.1", "127.0.0.2"}, nil
	case dns.IPv6:
		return []string{"::1", "::2"}, nil
	default:
		panic(dns.UnknownTypeError(opts.Type))
	}
}

type mockProxyPool struct {
	socks5Server *socks5.Server
	socks5Client *proxy.Client
	httpServer   *http.Server
	httpClient   *proxy.Client
}

func newMockProxyPool(t *testing.T) *mockProxyPool {
	mpp := mockProxyPool{}
	// start socks5 proxy server(s5s)
	s5sOpts := &socks5.Options{
		Username: "admin",
		Password: "123456",
	}
	s5s, err := socks5.NewServer("test_socks5", logger.Test, s5sOpts)
	require.NoError(t, err)
	err = s5s.ListenAndServe("localhost:0")
	require.NoError(t, err)
	mpp.socks5Server = s5s
	// create socks5 client(s5c)
	_, port, err := net.SplitHostPort(s5s.Address())
	require.NoError(t, err)
	mpp.socks5Client = &proxy.Client{
		Mode: proxy.Socks5,
		Config: []byte(`
        [[Clients]]
          Address = "localhost:` + port + `"
          Network = "tcp"
          Password = "123456"
          Username = "admin"
    `)}
	// start http proxy server(hps)
	hpsOpts := &http.Options{
		Username: "admin",
		Password: "123456",
	}
	hps, err := http.NewServer("test_http_proxy", logger.Test, hpsOpts)
	require.NoError(t, err)
	err = hps.ListenAndServe("localhost:0")
	require.NoError(t, err)
	mpp.httpServer = hps
	// create http proxy client(hc)
	_, port, err = net.SplitHostPort(hps.Address())
	require.NoError(t, err)
	mpp.httpClient = &proxy.Client{
		Mode:   proxy.HTTP,
		Config: []byte("http://admin:123456@localhost:" + port),
	}
	return &mpp
}

func (p *mockProxyPool) Get(tag string) (*proxy.Client, error) {
	switch tag {
	case "":
		return nil, nil
	case "http":
		return p.httpClient, nil
	case "socks5":
		return p.socks5Client, nil
	default:
		panic("doesn't exist")
	}
}

func (p *mockProxyPool) Close() {
	_ = p.socks5Server.Close()
	_ = p.httpServer.Close()
}

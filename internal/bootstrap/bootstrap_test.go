package bootstrap

import (
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/logger"
	"project/internal/netx"
	"project/internal/proxy"
	"project/internal/proxy/httpproxy"
	"project/internal/proxy/socks5"
)

func test_generate_nodes() []*Node {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{
		Mode:    netx.TLS,
		Network: "tcp",
		Address: "127.0.0.1:53123",
	}
	nodes[1] = &Node{
		Mode:    netx.TLS,
		Network: "tcp",
		Address: "192.168.1.11:53123",
	}
	return nodes
}

type mock_resolver struct{}

func (this *mock_resolver) Resolve(domain string, opts *dnsclient.Options) ([]string, error) {
	if domain != test_domain {
		return nil, errors.New("domain changed")
	}
	if opts == nil {
		opts = new(dnsclient.Options)
	}
	switch opts.Type {
	case "", dns.IPV4:
		return []string{"127.0.0.1", "192.168.1.11"}, nil
	case dns.IPV6:
		return []string{"::1", "fe80::5456:5f8:1690:5792"}, nil
	default:
		panic(dns.ERR_INVALID_TYPE)
	}
}

type mock_proxy_pool struct {
	socks5_server *socks5.Server
	socks5_client *proxyclient.Client
	http_server   *httpproxy.Server
	http_client   *proxyclient.Client
}

func (this *mock_proxy_pool) Init(t *testing.T) {
	// start socks5 proxy server(s5s)
	s5s_opts := &socks5.Options{
		Username: "admin",
		Password: "123456",
	}
	s5s, err := socks5.New_Server("test_socks5", logger.Test, s5s_opts)
	require.Nil(t, err, err)
	err = s5s.Listen_And_Serve("localhost:0", 0)
	require.Nil(t, err, err)
	this.socks5_server = s5s
	// create socks5 client(s5c)
	s5c, err := socks5.New_Client(&socks5.Config{
		Network:  "tcp",
		Address:  s5s.Addr(),
		Username: "admin",
		Password: "123456",
	})
	require.Nil(t, err, err)
	this.socks5_client = &proxyclient.Client{
		Mode:   proxy.SOCKS5,
		Client: s5c,
	}
	// start http proxy server(hs)
	hs_opts := &httpproxy.Options{
		Username: "admin",
		Password: "123456",
	}
	hs, err := httpproxy.New_Server("test_httpproxy", logger.Test, hs_opts)
	require.Nil(t, err, err)
	err = hs.Listen_And_Serve("localhost:0", 0)
	require.Nil(t, err, err)
	this.http_server = hs
	// create http proxy client(hc)
	hc, err := httpproxy.New_Client("http://admin:123456@" + hs.Addr())
	require.Nil(t, err, err)
	this.http_client = &proxyclient.Client{
		Mode:   proxy.HTTP,
		Client: hc,
	}
}

func (this *mock_proxy_pool) Close() {
	_ = this.socks5_server.Stop()
	_ = this.http_server.Stop()
}

func (this *mock_proxy_pool) Get(tag string) (*proxyclient.Client, error) {
	switch tag {
	case "":
		return nil, nil
	case "http":
		return this.http_client, nil
	case "socks5":
		return this.socks5_client, nil
	default:
		panic("doesn't exist")
	}
}

// return port
func test_start_http_server(t *testing.T, s *http.Server, info string) string {
	l, err := net.Listen("tcp", "")
	require.Nil(t, err, err)
	data := []byte(info)
	server_mux := http.NewServeMux()
	server_mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	s.Handler = server_mux
	err_chan := make(chan error, 1)
	go func() {
		if s.TLSConfig == nil {
			err_chan <- s.Serve(l)
		} else {
			err_chan <- s.ServeTLS(l, "", "")
		}
	}()
	// start
	select {
	case err := <-err_chan:
		t.Fatal(err)
	case <-time.After(250 * time.Millisecond):
	}
	//get port
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.Nil(t, err, err)
	return port
}

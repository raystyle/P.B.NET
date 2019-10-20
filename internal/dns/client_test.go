package dns

import (
	"io/ioutil"
	"net"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/options"
	"project/internal/proxy"
	"project/internal/proxy/http"
	"project/internal/proxy/socks5"
	"project/internal/testutil"
)

func TestClient(t *testing.T) {
	// make proxy pool
	proxyPool, err := proxy.NewPool(nil)
	require.NoError(t, err)
	defer func() {
		testutil.IsDestroyed(t, proxyPool, 1)
	}()
	// create dns servers
	servers := make(map[string]*Server)
	b, err := ioutil.ReadFile("testdata/dnsclient.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &servers)
	require.NoError(t, err)
	// make dns client
	client, err := NewClient(proxyPool, servers, options.DefaultCacheExpireTime)
	require.NoError(t, err)
	// delete dns server
	err = client.Delete("udp_google")
	require.NoError(t, err)
	// delete doesn't exist
	err = client.Delete("udp_google")
	require.Error(t, err)
	// print servers
	for tag, server := range client.Servers() {
		t.Log(tag, server)
	}
	// resolve with default options
	ipList, err := client.Resolve(domain, nil)
	require.NoError(t, err)
	t.Log("use default options", ipList)
	// resolve with tag
	opts := Options{ServerTag: "tcp_google"}
	ipList, err = client.Resolve(domain, &opts)
	require.NoError(t, err)
	t.Log("with tag", ipList)
	// client.FlushCache()
	client.FlushCache()
	testutil.IsDestroyed(t, client, 1)
}

const (
	proxySocks5 = "test_socks5_client"
	proxyHTTP   = "test_http_proxy_client"
)

func testGenerateProxyPool(t *testing.T) *proxy.Pool {
	// start socks5 proxy server(s5s)
	s5sOpts := &socks5.Options{
		Username: "admin",
		Password: "123456",
	}
	s5s, err := socks5.NewServer("test_socks5", logger.Test, s5sOpts)
	require.NoError(t, err)
	err = s5s.ListenAndServe("localhost:0")
	require.NoError(t, err)
	defer func() {
		err = s5s.Close()
		require.NoError(t, err)
	}()
	// start http proxy server(hps)
	hpsOpts := &http.Options{
		Username: "admin",
		Password: "123456",
	}
	hps, err := http.NewServer("test_http_proxy", logger.Test, hpsOpts)
	require.NoError(t, err)
	err = hps.ListenAndServe("localhost:0")
	require.NoError(t, err)
	defer func() {
		err = hps.Close()
		require.NoError(t, err)
	}()
	// create proxy clients
	proxyClients := make(map[string]*proxy.Client)
	// socks5
	_, port, err := net.SplitHostPort(hps.Address())
	require.NoError(t, err)
	proxyClients[proxySocks5] = &proxy.Client{
		Mode: proxy.Socks5,
		Config: []byte(`
        [[Clients]]
          Address = "localhost:` + port + `"
          Network = "tcp"
          Password = "123456"
          Username = "admin"
    `)}
	// http
	_, port, err = net.SplitHostPort(hps.Address())
	require.NoError(t, err)
	proxyClients[proxyHTTP] = &proxy.Client{
		Mode:   proxy.HTTP,
		Config: []byte("http://admin:123456@localhost:" + port),
	}
	return nil
}

func TestClient_Resolve(t *testing.T) {

}

/*
	//ipv4
	opt := &dns.Options{
		Method:  dns.DoT,
		Network: "tcp",
		Type:    dns.IPv4,
		Timeout: time.Second * 2,
	}
	ip_list, err := client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("DoT", ip_list)
	opt.Method = dns.TCP
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("TCP", ip_list)
	opt.Method = dns.UDP
	opt.Network = "udp"
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("UDP", ip_list)
	//ipv6
	opt = &dns.Options{
		Method:  dns.DoT,
		Network: "tcp",
		Type:    dns.IPv6,
		Timeout: time.Second * 2,
	}
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("DoT", ip_list)
	opt.Method = dns.TCP
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("TCP", ip_list)
	opt.Method = dns.UDP
	opt.Network = "udp"
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("UDP", ip_list)
	//invalid expire
	_, err = New(-1, nil)
	require.NoError(t, err)
	//not exist domain
	opt.Method = dns.TCP
	opt.Network = "tcp"
	ip_list, err = client_pool.Resolve("asdasdasf1561asf651af.com", opt)
	require.Equal(t, err, dns.ErrNoResolveResult, err)
	//flush cache
	client_pool.FlushCache()
	opt.Method = dns.DoT
	opt.Network = "tcp"
	opt.Type = dns.IPv6
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("flush cache IPv6", ip_list)
	opt.Type = dns.IPv4
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("flush cache IPv4", ip_list)
	//expire
	require.NotNil(t, client_pool.SetCacheExpireTime(-1))
	require.Nil(t, client_pool.SetCacheExpireTime(1))
	time.Sleep(time.Second * 2)
	ip_list, err = client_pool.Resolve(domain, opt)
	require.NoError(t, err)
	t.Log("cache expire IPv4", ip_list)
	//update cache
	client_pool.update_cache("xxx.com", nil, nil)
	//delete
	for tag := range pool {
		err = client_pool.Delete(tag)
		require.NoError(t, err)
		err = client_pool.Delete(tag)
		require.NoError(t, err)
	}
*/

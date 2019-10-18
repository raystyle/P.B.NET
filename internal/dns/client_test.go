package dns

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/http"
	"project/internal/proxy/socks5"
)

const (
	proxySocks5 = "test_socks5_client"
	proxyHTTP   = "test_http_proxy_client"
)

func TestClient(t *testing.T) {
	// start socks5 proxy server(s5s)
	s5sOpts := &socks5.Options{
		Username: "admin",
		Password: "123456",
	}
	s5s, err := socks5.NewServer("test_socks5", logger.Test, s5sOpts)
	require.NoError(t, err)
	err = s5s.ListenAndServe("localhost:0", 0)
	require.NoError(t, err)
	defer func() {
		err = s5s.Stop()
		require.NoError(t, err)
	}()
	// start http proxy server(hps)
	hpsOpts := &http.Options{
		Username: "admin",
		Password: "123456",
	}
	hps, err := http.NewServer("test_http_proxy", logger.Test, hpsOpts)
	require.NoError(t, err)
	err = hps.ListenAndServe("localhost:0", 0)
	require.NoError(t, err)
	defer func() {
		err = hps.Stop()
		require.NoError(t, err)
	}()
	// create proxy clients
	proxyClients := make(map[string]*proxy.Client)
	// socks5
	_, port, err := net.SplitHostPort(hps.Addr())
	require.NoError(t, err)
	proxyClients[proxySocks5] = &proxy.Client{
		Mode: proxy.Socks5,
		Config: `
        [[Clients]]
          Address = "localhost:` + port + `"
          Network = "tcp"
          Password = "123456"
          Username = "admin"
    `}
	// http
	_, port, err = net.SplitHostPort(hps.Addr())
	require.NoError(t, err)
	proxyClients[proxyHTTP] = &proxy.Client{
		Mode:   proxy.HTTP,
		Config: "http://admin:123456@localhost:" + port,
	}
	// make proxy pool
	pool, err := proxy.NewPool(proxyClients)
	require.NoError(t, err)
	// create dns servers
	servers := make(map[string]*Server)
	add := func(tag string, method Method, address string) {
		servers[tag] = &Server{
			Method:  method,
			Address: address,
		}
	}
	// google
	add("udp_google", UDP, "8.8.8.8:53")
	add("tcp_google", TCP, "8.8.8.8:53")
	add("dot_google_domain", DoT, "dns.google:853|8.8.8.8,8.8.4.4")
	// cloudflare
	add("udp_cloudflare", UDP, "1.0.0.1:53")
	add("tcp_cloudflare_ipv6", TCP, "[2606:4700:4700::1001]:53")
	add("dot_cloudflare_domain", DoT, "cloudflare-dns.com:853|1.0.0.1")
	// doh
	add("doh_mozilla", DoH, "https://mozilla.cloudflare-dns.com/dns-query")
	// make dns client
	client, err := NewClient(pool, servers, time.Minute)
	require.NoError(t, err)
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

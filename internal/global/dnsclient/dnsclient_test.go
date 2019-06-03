package dnsclient

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/global/proxyclient"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/httpproxy"
	"project/internal/proxy/socks5"
)

const (
	proxy_socks5 = "test_socks5_client"
	proxy_http   = "test_http_proxy_client"
)

const domain = "ipv6.baidu.com"

func Test_DNS(t *testing.T) {
	// start socks5 proxy server(s5s)
	s5s_opts := &socks5.Options{
		Username: "admin",
		Password: "123456",
	}
	s5s_server, err := socks5.New_Server("test_socks5", logger.Test, s5s_opts)
	require.Nil(t, err, err)
	err = s5s_server.Listen_And_Serve("localhost:0", 0)
	require.Nil(t, err, err)
	defer func() {
		err = s5s_server.Stop()
		require.Nil(t, err, err)
	}()
	// start http proxy server(hs)
	hs_opts := &httpproxy.Options{
		Username: "admin",
		Password: "123456",
	}
	hs_server, err := httpproxy.New_Server("test_httpproxy", logger.Test, hs_opts)
	require.Nil(t, err, err)
	err = hs_server.Listen_And_Serve("localhost:0", 0)
	require.Nil(t, err, err)
	defer func() {
		err = hs_server.Stop()
		require.Nil(t, err, err)
	}()
	// create proxy clients
	p_clients := make(map[string]*proxyclient.Client)
	// create socks5 client config(s5c)
	s5cc := &socks5.Config{
		Network:  "tcp",
		Address:  "localhost:0",
		Username: "admin",
		Password: "123456",
	}
	p_clients[proxy_socks5] = &proxyclient.Client{
		Mode:   proxy.SOCKS5,
		Config: []*socks5.Config{s5cc},
	}
	p_clients[proxy_http] = &proxyclient.Client{
		Mode:   proxy.HTTP,
		Config: "http://admin:123456@localhost:0",
	}
	// make PROXY
	PROXY, err := proxyclient.New(p_clients)
	require.Nil(t, err, err)
	// make DNS
	// create dns clients
	clients := make(map[string]*Client)
	add := func(tag string, method dns.Method, address string) {
		clients[tag] = &Client{
			Method:  method,
			Address: address,
		}
	}
	// google
	add("udp_google", dns.UDP, "8.8.8.8:53")
	add("tcp_google", dns.TCP, "8.8.8.8:53")
	add("tls_google_domain", dns.TLS, "dns.google:853|8.8.8.8,8.8.4.4")
	// cloudflare
	add("udp_cloudflare", dns.UDP, "1.0.0.1:53")
	add("tcp_cloudflare_ipv6", dns.TCP, "[2606:4700:4700::1001]:53")
	add("tls_cloudflare_domain", dns.TLS, "cloudflare-dns.com:853|1.0.0.1")
	// doh
	add("doh_mozilla", dns.DOH, "https://mozilla.cloudflare-dns.com/dns-query")
	DNS, err := New(PROXY, clients, 0)
	require.Nil(t, err, err)
	// resolve with default options
	ip_list, err := DNS.Resolve(domain, nil)
	require.Nil(t, err, err)
	t.Log("use default options", ip_list)
	/*
		//ipv4
		opt := &dns.Options{
			Method:  dns.TLS,
			Network: "tcp",
			Type:    dns.IPV4,
			Timeout: time.Second * 2,
		}
		ip_list, err := client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("TLS", ip_list)
		opt.Method = dns.TCP
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("TCP", ip_list)
		opt.Method = dns.UDP
		opt.Network = "udp"
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("UDP", ip_list)
		//ipv6
		opt = &dns.Options{
			Method:  dns.TLS,
			Network: "tcp",
			Type:    dns.IPV6,
			Timeout: time.Second * 2,
		}
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("TLS", ip_list)
		opt.Method = dns.TCP
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("TCP", ip_list)
		opt.Method = dns.UDP
		opt.Network = "udp"
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("UDP", ip_list)
		//invalid deadline
		_, err = New(-1, nil)
		require.NotNil(t, err)
		//not exist domain
		opt.Method = dns.TCP
		opt.Network = "tcp"
		ip_list, err = client_pool.Resolve("asdasdasf1561asf651af.com", opt)
		require.Equal(t, err, dns.ERR_NO_RESOLVE_RESULT, err)
		//flush cache
		client_pool.Flush_Cache()
		opt.Method = dns.TLS
		opt.Network = "tcp"
		opt.Type = dns.IPV6
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("flush cache IPv6", ip_list)
		opt.Type = dns.IPV4
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("flush cache IPv4", ip_list)
		//deadline
		require.NotNil(t, client_pool.Set_Cache_Deadline(-1))
		require.Nil(t, client_pool.Set_Cache_Deadline(1))
		time.Sleep(time.Second * 2)
		ip_list, err = client_pool.Resolve(domain, opt)
		require.Nil(t, err, err)
		t.Log("cache deadline IPv4", ip_list)
		//update cache
		client_pool.update_cache("asdasd.com", nil, nil)
		//delete
		for tag := range pool {
			err = client_pool.Delete(tag)
			require.Nil(t, err, err)
			err = client_pool.Delete(tag)
			require.NotNil(t, err)
		}
	*/
	DNS.Destroy()
}

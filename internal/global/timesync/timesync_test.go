package timesync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/global/dnsclient"
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

func Test_TIMESYNC(t *testing.T) {
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
	d_clients := make(map[string]*dnsclient.Client)
	add := func(tag string, method dns.Method, address string) {
		d_clients[tag] = &dnsclient.Client{
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
	DNS, err := dnsclient.New(PROXY, d_clients, 0)
	require.Nil(t, err, err)
	// create timesync clients
	clients := make(map[string]*Client)
	clients["test_baidu"] = &Client{
		Address: "https://www.baidu.com/",
	}
	TIMESYNC, err := New(PROXY, DNS, logger.Test, clients, 0)
	require.Nil(t, err, err)
	err = TIMESYNC.Start()
	require.Nil(t, err, err)
	time.Sleep(3 * time.Second)
	t.Log("now:", TIMESYNC.Now())
	for k, v := range TIMESYNC.Clients() {
		t.Log("client:", k, v.Mode, v.Address)
	}
	TIMESYNC.Destroy()
}

/*

	ntp_client_pool := make(map[string]*Client)
	ntp_client_pool["pool.ntp.org"] = &Client{
		Address:     "pool.ntp.org:123",
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.TLS,
			Type:   dns.IPV4,
		},
	}
	ntp_client_pool["0.pool.ntp.org"] = &Client{
		Address:     "0.pool.ntp.org:123",
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.TLS,
			Type:   dns.IPV4,
		},
	}
	ntp_client_pool["time.windows.com"] = &Client{
		Address:     "time.windows.com:123",
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.TLS,
			Type:   dns.IPV4,
		},
	}
	return ntp_client_pool


func Test_NTP_Client_Pool(t *testing.T) {
	//init dns client pool
	dns_client_pool, err := dnsclient.New_Pool(15, nil)
	require.Nil(t, err, err)
	for tag, client := range dnsclient.Test_Generate_DNS_Client() {
		dns_client_pool.Add(tag, client)
	}
	//init ntp client pool
	ntp_client_pool, err := New_Pool(time.Minute, dns_client_pool)
	require.Nil(t, err, err)
	clients := Test_Generate_NTP_Client()
	//for test interval
	require.Nil(t, ntp_client_pool.Set_Interval(time.Minute))
	ntp_client_pool.lock.Lock()
	ntp_client_pool.sync_interval = time.Millisecond * 500
	ntp_client_pool.lock.Unlock()
	go func() { //wait add
		time.Sleep(time.Second)
		for tag, client := range clients {
			err := ntp_client_pool.Add(tag, client)
			require.Nil(t, err, err)
			err = ntp_client_pool.Add(tag, client)
			require.NotNil(t, err)
		}
	}()
	ntp_client_pool.Start()
	t.Log("now", ntp_client_pool.Now())
	//for add
	time.Sleep(time.Second)
	//delete
	for tag := range clients {
		err := ntp_client_pool.Delete(tag)
		require.Nil(t, err, err)
		err = ntp_client_pool.Delete(tag)
		require.NotNil(t, err)
	}
	//invalid interval
	_, err = New_Pool(0, dns_client_pool)
	require.NotNil(t, err)
	require.NotNil(t, ntp_client_pool.Set_Interval(time.Second))
	//invalid address
	ntp_client_pool.Add("client_i1", &Client{
		Address:     "asdadasd", //no port
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{},
	})
	ntp_client_pool.Destroy()
	dns_client_pool.Destroy()
}

func Test_sync_time(t *testing.T) {
	//init dns client pool
	dns_client_pool, err := dnsclient.New_Pool(15, nil)
	require.Nil(t, err, err)
	for tag, client := range dnsclient.Test_Generate_DNS_Client() {
		dns_client_pool.Add(tag, client)
	}
	//init ntp client pool
	ntp_client_pool, err := New_Pool(time.Minute, dns_client_pool)
	require.Nil(t, err, err)
	//invalid ntp server
	client_i1 := &Client{
		Address:     "poasdasdol.ntp.orasdasd:123", //this
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.TLS,
			Type:   dns.IPV4,
		},
	}
	ntp_client_pool.Add("client_i1", client_i1)
	require.False(t, ntp_client_pool.sync())
	t.Log("invalid ntp server ", ntp_client_pool.Now())
	ntp_client_pool.Delete("client_i1")
	//invalid ntp options
	client_i2 := &Client{
		Address:     "pool.ntp.org:123",
		NTP_Options: &ntp.Options{Version: 5}, //this
		DNS_Options: &dns.Options{
			Method: dns.TLS,
			Type:   dns.IPV4,
		},
	}
	ntp_client_pool.Add("client_i2", client_i2)
	require.False(t, ntp_client_pool.sync())
	t.Log("invalid ntp options", ntp_client_pool.Now())
	ntp_client_pool.Delete("client_i2")
}
*/

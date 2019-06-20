package node

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
)

const (
	proxy_socks5 = "test_socks5_client"
	proxy_http   = "test_http_proxy_client"
)

func Test_Node(t *testing.T) {
	config := test_generate_config(t)
	//err := config.Check()
	//require.Nil(t, err, err)
	node, err := New(config)
	require.Nil(t, err, err)

	for k := range node.global.proxy.Clients() {
		t.Log("proxy client:", k)
	}

	for k := range node.global.dns.Clients() {
		t.Log("dns client:", k)
	}

	return
	err = node.Main()
	require.Nil(t, err, err)
}

func test_generate_config(t *testing.T) *Config {
	c := &Config{
		Log_level: "debug",
	}
	// load proxy clients
	proxy_clients := make(map[string]*proxyclient.Client)
	b, err := ioutil.ReadFile("../config/proxyclient.toml")
	require.Nil(t, err, err)
	err = toml.Unmarshal(b, &proxy_clients)
	require.Nil(t, err, err)
	// load dns clients
	dns_clients := make(map[string]*dnsclient.Client)
	b, err = ioutil.ReadFile("../config/dnsclient.toml")
	require.Nil(t, err, err)
	err = toml.Unmarshal(b, &dns_clients)
	require.Nil(t, err, err)

	/*
			// create proxy clients
			proxy_clients := make(map[string]*proxyclient.Client)
			proxy_clients[proxy_socks5] = &proxyclient.Client{
				Mode: proxy.SOCKS5,
				Config: `
		        [[Clients]]
		          Address = "localhost:0"
		          Network = "tcp"
		          Password = "123456"
		          Username = "admin"

		        [[Clients]]
		          Address = "localhost:0"
		          Network = "tcp"
		          Password = "123456"
		          Username = "admin"
		`,
			}
			proxy_clients[proxy_http] = &proxyclient.Client{
				Mode:   proxy.HTTP,
				Config: "http://admin:123456@localhost:0",
			}


		// create dns client

		add := func(tag string, method dns.Method, address string) {
			dns_clients[tag] = &dnsclient.Client{
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

		conf, _ := toml.Marshal(dns_clients)
		fmt.Println(string(conf))
	*/
	// create time sync client
	timesync_clients := make(map[string]*timesync.Client)
	timesync_clients["test_http"] = &timesync.Client{
		Mode:    timesync.HTTP,
		Address: "https://www.baidu.com/",
	}
	timesync_clients["test_ntp"] = &timesync.Client{
		Mode:    timesync.NTP,
		Address: "pool.ntp.org:123",
	}

	c.Proxy_Clients = proxy_clients
	c.DNS_Clients = dns_clients
	c.DNS_Cache_Deadline = 3 * time.Minute
	c.Timesync_Clients = timesync_clients
	c.Timesync_Interval = 15 * time.Minute
	return c
}

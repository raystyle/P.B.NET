package testdata

import (
	"encoding/pem"
	"io/ioutil"
	"sync"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/socks"
	"project/internal/testsuite"
	"project/internal/timesync"
)

// Certificates is used to provide CA certificates for test,
// certificates are from Windows
func Certificates(t require.TestingT) [][]byte {
	pemBlock, err := ioutil.ReadFile("../testdata/system.pem")
	require.NoError(t, err)
	var certs [][]byte // ASN1 data
	var block *pem.Block
	for {
		block, pemBlock = pem.Decode(pemBlock)
		require.NotNil(t, block)
		certs = append(certs, block.Bytes)
		if len(pemBlock) == 0 {
			break
		}
	}
	return certs
}

var (
	initProxyClientsOnce sync.Once
	proxyServer          *socks.Server
)

// ProxyClients is used to deploy a proxy server
// and return corresponding proxy client
func ProxyClients(t require.TestingT) []*proxy.Client {
	initProxyClientsOnce.Do(func() {
		var err error
		proxyServer, err = socks.NewServer("test", logger.Test, nil)
		require.NoError(t, err)
		err = proxyServer.ListenAndServe("tcp", "localhost:0")
		require.NoError(t, err)
	})
	return []*proxy.Client{
		{
			Tag:     "test_socks5",
			Mode:    proxy.ModeSocks,
			Network: "tcp",
			Address: proxyServer.Address(),
		},
	}
}

// DNSServers is used to provide DNS servers for test
func DNSServers() map[string]*dns.Server {
	servers := make(map[string]*dns.Server)
	if testsuite.IPv4Enabled {
		servers["test_udp_ipv4"] = &dns.Server{
			Method:  "udp",
			Address: "8.8.8.8:53",
		}
		servers["test_tcp_ipv4"] = &dns.Server{
			Method:  "tcp",
			Address: "8.8.8.8:53",
		}
		servers["test_dot_ipv4"] = &dns.Server{
			Method:  "dot",
			Address: "1.1.1.1:853",
		}
		servers["test_skip_ipv4"] = &dns.Server{
			Method:   "udp",
			Address:  "1.1.1.1:53",
			SkipTest: true,
		}
	}
	if testsuite.IPv6Enabled {
		servers["test_udp_ipv6"] = &dns.Server{
			Method:  "udp",
			Address: "[2606:4700:4700::1111]:53",
		}
		servers["test_tcp_ipv6"] = &dns.Server{
			Method:  "tcp",
			Address: "[2606:4700:4700::1111]:53",
		}
		servers["test_dot_ipv6"] = &dns.Server{
			Method:  "dot",
			Address: "[2606:4700:4700::1111]:853",
		}
		servers["test_skip_ipv6"] = &dns.Server{
			Method:   "udp",
			Address:  "[2606:4700:4700::1001]:53",
			SkipTest: true,
		}
	}
	servers["test_doh_ds"] = &dns.Server{
		Method:  "doh",
		Address: "https://cloudflare-dns.com/dns-query",
	}
	return servers
}

// TimeSyncerClients is used to provide time syncer clients for test
func TimeSyncerClients() map[string]*timesync.Client {
	clients := make(map[string]*timesync.Client)
	config := `
timeout = "15s"

[request]
  url   = "https://www.cloudflare.com/"
  close = true

  [request.header]
    User-Agent      = ["Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:72.0) Gecko/20100101 Firefox/72.0"]
    Accept          = ["text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"]
    Accept-Language = ["zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2"]
    DNT             = ["1"]
    Pragma          = ["no-cache"]
    Cache-Control   = ["no-cache"]

[transport]

  [transport.tls_config]
    insecure_load_from_system = true
`
	clients["test_http"] = &timesync.Client{
		Mode:   timesync.ModeHTTP,
		Config: config,
	}
	config = `address = "2.pool.ntp.org:123"`
	clients["test_ntp"] = &timesync.Client{
		Mode:   timesync.ModeNTP,
		Config: config,
	}
	return clients
}

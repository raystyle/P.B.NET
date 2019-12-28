package testdata

import (
	"encoding/pem"
	"io/ioutil"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/proxy/socks"
	"project/internal/testsuite"
	"project/internal/timesync"
)

// Certificates is used to provide CA certificate for test
func Certificates(t require.TestingT) [][]byte {
	pemBlock, err := ioutil.ReadFile("../testdata/system.pem")
	require.NoError(t, err)
	var ASN1 [][]byte
	var block *pem.Block
	for {
		block, pemBlock = pem.Decode(pemBlock)
		require.NotNil(t, block)
		ASN1 = append(ASN1, block.Bytes)
		if len(pemBlock) == 0 {
			break
		}
	}
	return ASN1
}

// ProxyClients is used to deploy a test proxy server
// and return corresponding proxy client
func ProxyClients(t require.TestingT) []*proxy.Client {
	server, err := socks.NewServer("test", logger.Test, nil)
	require.NoError(t, err)
	err = server.ListenAndServe("tcp", "localhost:0")
	require.NoError(t, err)
	return []*proxy.Client{
		{
			Tag:     "test_socks5",
			Mode:    proxy.ModeSocks,
			Network: "tcp",
			Address: server.Address(),
		},
	}
}

// DNSServers is used to provide test DNS servers
func DNSServers() map[string]*dns.Server {
	servers := make(map[string]*dns.Server)
	if testsuite.EnableIPv4() {
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
	if testsuite.EnableIPv6() {
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

// TimeSyncerClients is used to provide test time syncer clients
func TimeSyncerClients(t require.TestingT) map[string]*timesync.Client {
	clients := make(map[string]*timesync.Client)
	config, err := ioutil.ReadFile("../internal/timesync/testdata/http.toml")
	require.NoError(t, err)
	clients["test_http"] = &timesync.Client{
		Mode:   timesync.ModeHTTP,
		Config: string(config),
	}
	config, err = ioutil.ReadFile("../internal/timesync/testdata/ntp.toml")
	require.NoError(t, err)
	clients["test_ntp"] = &timesync.Client{
		Mode:   timesync.ModeNTP,
		Config: string(config),
	}
	return clients
}

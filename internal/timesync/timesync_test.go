package timesync

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/proxy"
)

func TestTimeSyncer(t *testing.T) {
	proxyPool, err := proxy.NewPool(nil)
	require.NoError(t, err)
	dnsClient := testGenerateDNSClient(t, proxyPool)
	// create clients
	clients := make(map[string]*Client)
	clients["http"] = &Client{
		Mode:   HTTP,
		Config: testNewHTTPClient(t).ExportConfig(),
	}
	clients["ntp"] = &Client{
		Mode:   NTP,
		Config: testNewNTPClient(t).ExportConfig(),
	}
	timeSyncer, err := NewTimeSyncer(
		proxyPool,
		dnsClient,
		logger.Test,
		clients,
		options.DefaultSyncInterval,
	)
	require.NoError(t, err)
	err = timeSyncer.Test()
	require.NoError(t, err)
	err = timeSyncer.Start()
	require.NoError(t, err)
	time.Sleep(3 * time.Second)
	t.Log("now:", timeSyncer.Now())
	for tag, client := range timeSyncer.Clients() {
		t.Logf("client: %s mode: %s \nconfig: %s",
			tag, client.Mode, string(client.ExportConfig()))
	}
	timeSyncer.Stop()
}

func testGenerateDNSClient(t *testing.T, pool *proxy.Pool) *dns.Client {
	servers := make(map[string]*dns.Server)
	b, err := ioutil.ReadFile("testdata/dnsclient.toml")
	require.NoError(t, err)
	err = toml.Unmarshal(b, &servers)
	require.NoError(t, err)
	// make dns client
	dnsClient, err := dns.NewClient(pool, servers, options.DefaultCacheExpireTime)
	require.NoError(t, err)
	return dnsClient
}

/*

const (
	proxySocks5 = "test_socks5_client"
	proxyHTTP   = "test_http_proxy_client"
)

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
*/

/*

	ntp_client_pool := make(map[string]*client)
	ntp_client_pool["pool.ntp.org"] = &client{
		Address:     "pool.ntp.org:123",
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.DoT,
			Type:   dns.IPv4,
		},
	}
	ntp_client_pool["0.pool.ntp.org"] = &client{
		Address:     "0.pool.ntp.org:123",
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.DoT,
			Type:   dns.IPv4,
		},
	}
	ntp_client_pool["time.windows.com"] = &client{
		Address:     "time.windows.com:123",
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.DoT,
			Type:   dns.IPv4,
		},
	}
	return ntp_client_pool


func Test_NTP_Client_Pool(t *testing.T) {
	//init dns client pool
	dns_client_pool, err := dnsclient.New_Pool(15, nil)
	require.NoError(t, err)
	for tag, client := range dnsclient.Test_Generate_DNS_Client() {
		dns_client_pool.Add(tag, client)
	}
	//init ntp client pool
	ntp_client_pool, err := New_Pool(time.Minute, dns_client_pool)
	require.NoError(t, err)
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
			require.NoError(t, err)
			err = ntp_client_pool.Add(tag, client)
			require.NoError(t, err)
		}
	}()
	ntp_client_pool.Start()
	t.Log("now", ntp_client_pool.Now())
	//for add
	time.Sleep(time.Second)
	//delete
	for tag := range clients {
		err := ntp_client_pool.Delete(tag)
		require.NoError(t, err)
		err = ntp_client_pool.Delete(tag)
		require.NoError(t, err)
	}
	//invalid interval
	_, err = New_Pool(0, dns_client_pool)
	require.NoError(t, err)
	require.NotNil(t, ntp_client_pool.Set_Interval(time.Second))
	//invalid address
	ntp_client_pool.Add("client_i1", &client{
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
	require.NoError(t, err)
	for tag, client := range dnsclient.Test_Generate_DNS_Client() {
		dns_client_pool.Add(tag, client)
	}
	//init ntp client pool
	ntp_client_pool, err := New_Pool(time.Minute, dns_client_pool)
	require.NoError(t, err)
	//invalid ntp server
	client_i1 := &client{
		Address:     "poasdasdol.ntp.orasdasd:123", //this
		NTP_Options: &ntp.Options{},
		DNS_Options: &dns.Options{
			Method: dns.DoT,
			Type:   dns.IPv4,
		},
	}
	ntp_client_pool.Add("client_i1", client_i1)
	require.False(t, ntp_client_pool.sync())
	t.Log("invalid ntp server ", ntp_client_pool.Now())
	ntp_client_pool.Delete("client_i1")
	//invalid ntp options
	client_i2 := &client{
		Address:     "pool.ntp.org:123",
		NTP_Options: &ntp.Options{Version: 5}, //this
		DNS_Options: &dns.Options{
			Method: dns.DoT,
			Type:   dns.IPv4,
		},
	}
	ntp_client_pool.Add("client_i2", client_i2)
	require.False(t, ntp_client_pool.sync())
	t.Log("invalid ntp options", ntp_client_pool.Now())
	ntp_client_pool.Delete("client_i2")
}
*/

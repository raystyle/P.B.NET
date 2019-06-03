package timesync

import (
	"testing"
)

func Test_TIMESYNC(t *testing.T) {

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
	require.False(t, ntp_client_pool.sync_time())
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
	require.False(t, ntp_client_pool.sync_time())
	t.Log("invalid ntp options", ntp_client_pool.Now())
	ntp_client_pool.Delete("client_i2")
}
*/

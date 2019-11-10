package timesync

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/testsuite/testdns"
)

func TestNTPClient_Query(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	NTP := NewNTP(pool, dnsClient)
	b, err := ioutil.ReadFile("testdata/ntp_opts.toml")
	require.NoError(t, err)
	require.NoError(t, NTP.Import(b))

	// simple query
	now, optsErr, err := NTP.Query()
	require.NoError(t, err)
	require.False(t, optsErr)
	t.Log("now(NTP) simple:", now.Local())
}

func TestNTPClient_Query_Failed(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	NTP := NewNTP(pool, dnsClient)
	b, err := ioutil.ReadFile("testdata/ntp_opts.toml")
	require.NoError(t, err)
	require.NoError(t, NTP.Import(b))

	// invalid network
	NTP.Network = "foo network"
	_, optsErr, err := NTP.Query()
	require.Error(t, err)
	require.True(t, optsErr)

	NTP.Network = ""

	// invalid address
	NTP.Address = "foo address"
	_, optsErr, err = NTP.Query()
	require.Error(t, err)
	require.True(t, optsErr)

	// invalid domain
	NTP.Address = "foo1516ads.com:123"
	_, optsErr, err = NTP.Query()
	require.Error(t, err)
	require.True(t, optsErr)

	// all failed
	NTP.Address = "github.com:8989"
	NTP.Timeout = time.Second
	_, optsErr, err = NTP.Query()
	require.Error(t, err)
	require.False(t, optsErr)
}

func TestNTPOptions(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/ntp.toml")
	require.NoError(t, err)
	require.NoError(t, TestNTP(b))
	NTP := new(NTP)
	require.NoError(t, NTP.Import(b))

	testdata := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "udp4", actual: NTP.Network},
		{expected: "1.2.3.4:123", actual: NTP.Address},
		{expected: 15 * time.Second, actual: NTP.Timeout},
		{expected: 4, actual: NTP.Version},
		{expected: dns.ModeSystem, actual: NTP.DNSOpts.Mode},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}

	// export
	export := NTP.Export()
	require.NotEqual(t, 0, len(export))
	t.Log(string(export))
	require.NoError(t, NTP.Import(export))
}

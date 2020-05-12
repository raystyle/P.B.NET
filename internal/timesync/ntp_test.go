package timesync

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
)

func TestNTP_Query(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, _ := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	NTP := NewNTP(context.Background(), proxyPool, dnsClient)
	data, err := ioutil.ReadFile("testdata/ntp.toml")
	require.NoError(t, err)
	require.NoError(t, NTP.Import(data))

	// simple query
	now, optsErr, err := NTP.Query()
	require.NoError(t, err)
	require.False(t, optsErr)
	t.Log("now(NTP):", now.Local())

	testsuite.IsDestroyed(t, NTP)
}

func TestNTP_Query_Failed(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, _ := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("invalid network", func(t *testing.T) {
		NTP := NewNTP(context.Background(), proxyPool, dnsClient)

		NTP.Network = "foo network"

		_, optsErr, err := NTP.Query()
		require.Error(t, err)
		require.True(t, optsErr)

		testsuite.IsDestroyed(t, NTP)
	})

	t.Run("invalid address", func(t *testing.T) {
		NTP := NewNTP(context.Background(), proxyPool, dnsClient)

		NTP.Address = "foo address"

		_, optsErr, err := NTP.Query()
		require.Error(t, err)
		require.True(t, optsErr)

		testsuite.IsDestroyed(t, NTP)
	})

	t.Run("invalid domain", func(t *testing.T) {
		NTP := NewNTP(context.Background(), proxyPool, dnsClient)

		NTP.Address = "test:123"

		_, optsErr, err := NTP.Query()
		require.Error(t, err)
		require.True(t, optsErr)

		testsuite.IsDestroyed(t, NTP)
	})

	t.Run("all failed", func(t *testing.T) {
		NTP := NewNTP(context.Background(), proxyPool, dnsClient)

		NTP.Address = "github.com:8989"
		NTP.Timeout = time.Second

		_, optsErr, err := NTP.Query()
		require.Error(t, err)
		require.False(t, optsErr)

		testsuite.IsDestroyed(t, NTP)
	})
}

func TestNTPOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/ntp_opts.toml")
	require.NoError(t, err)

	err = TestNTP(data)
	require.NoError(t, err)

	NTP := new(NTP)
	err = NTP.Import(data)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, NTP)

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
	require.NotEmpty(t, export)
	t.Log(string(export))

	err = NTP.Import(export)
	require.NoError(t, err)
}

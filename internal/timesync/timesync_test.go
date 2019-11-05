package timesync

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite/testdns"
)

func testAddClients(t *testing.T, syncer *Syncer) {
	// add http
	b, err := ioutil.ReadFile("testdata/http_opts.toml")
	require.NoError(t, err)
	err = syncer.Add("http", &Client{
		Mode:   ModeHTTP,
		Config: b,
	})
	require.NoError(t, err)

	// add ntp
	b, err = ioutil.ReadFile("testdata/ntp_opts.toml")
	require.NoError(t, err)
	err = syncer.Add("ntp", &Client{
		Mode:   ModeNTP,
		Config: b,
	})
	require.NoError(t, err)
}

func TestTimeSyncer(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	syncer := New(pool, dnsClient, logger.Test)
	testAddClients(t, syncer)
	require.NoError(t, syncer.Start())
	t.Log("now: ", syncer.Now().Local())
}

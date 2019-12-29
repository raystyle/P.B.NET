package timesync

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
)

func testAddHTTP(t *testing.T, syncer *Syncer) {
	b, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	err = syncer.Add("http", &Client{
		Mode:   ModeHTTP,
		Config: string(b),
	})
	require.NoError(t, err)
}

func testAddNTP(t *testing.T, syncer *Syncer) {
	b, err := ioutil.ReadFile("testdata/ntp.toml")
	require.NoError(t, err)
	err = syncer.Add("ntp", &Client{
		Mode:   ModeNTP,
		Config: string(b),
	})
	require.NoError(t, err)
}

func testAddClients(t *testing.T, syncer *Syncer) {
	testAddHTTP(t, syncer)
	testAddNTP(t, syncer)
}

func TestSyncer(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	syncer := New(pool, dnsClient, logger.Test)
	testAddClients(t, syncer)

	// check default sync interval
	require.Equal(t, defaultTimeSyncInterval, syncer.GetSyncInterval())

	// set sync interval
	const interval = 15 * time.Minute
	require.NoError(t, syncer.SetSyncInterval(interval))
	require.Equal(t, interval, syncer.GetSyncInterval())

	// set invalid sync interval
	require.Error(t, syncer.SetSyncInterval(3*time.Hour))
	require.NoError(t, syncer.Start())
	t.Log("now: ", syncer.Now().Local())

	// wait walker
	time.Sleep(3 * time.Second)
	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func testUnreachableClient() *Client {
	return &Client{
		Mode: ModeNTP,
		Config: `
           network = "udp"
           address = "0.0.0.0:12"
           timeout = "1s"         `,
	}
}

func TestSyncer_Start(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	syncer := New(pool, dnsClient, logger.Test)
	// set random sleep
	syncer.SetSleep(0, 0)
	syncer.SetSleep(3, 5)

	// no clients
	require.Equal(t, ErrNoClients, syncer.Start())

	// invalid config
	err := syncer.Add("invalid config", &Client{
		Mode:   ModeNTP,
		Config: `network = "foo network"`,
	})
	require.NoError(t, err)
	require.Error(t, syncer.Start())

	require.NoError(t, syncer.Delete("invalid config"))

	// test all failed
	go func() {
		// delete unreachable
		time.Sleep(time.Second)
		require.NoError(t, syncer.Delete("unreachable"))

		// add reachable
		testAddHTTP(t, syncer)
	}()

	// add unreachable server
	require.NoError(t, syncer.Add("unreachable", testUnreachableClient()))
	require.NoError(t, syncer.Start())
	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_StartWalker(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	syncer := New(pool, dnsClient, logger.Test)
	syncer.StartWalker()
	now := syncer.Now()
	time.Sleep(2 * time.Second)
	require.False(t, syncer.Now().Equal(now))
}

func TestSyncer_Add_Delete(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	syncer := New(pool, dnsClient, logger.Test)
	testAddClients(t, syncer)

	// add unknown mode
	err := syncer.Add("foo mode", &Client{Mode: "foo mode"})
	require.Error(t, err)

	// invalid config
	err = syncer.Add("invalid config", &Client{
		Mode:   ModeNTP,
		Config: string([]byte{1, 2, 3, 4}),
	})
	require.Error(t, err)

	// add exist
	err = syncer.Add("ntp", &Client{
		Mode: ModeNTP,
	})
	require.Error(t, err)

	// delete
	require.NoError(t, syncer.Delete("http"))
	require.Error(t, syncer.Delete("http"))

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Test(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()
	syncer := New(pool, dnsClient, logger.Test)

	// no clients
	require.Equal(t, ErrNoClients, syncer.Test(context.Background()))

	// add reachable
	testAddHTTP(t, syncer)

	// add skip
	err := syncer.Add("skip", &Client{
		Mode:     ModeNTP,
		SkipTest: true,
	})
	require.NoError(t, err)

	// test
	require.NoError(t, syncer.Test(context.Background()))

	// test failed
	require.NoError(t, syncer.Delete("http"))
	err = syncer.Add("unreachable", testUnreachableClient())
	require.NoError(t, err)
	require.Error(t, syncer.Test(context.Background()))

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_synchronizeLoop(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	syncer := New(pool, dnsClient, logger.Test)

	// force set synchronize interval
	syncer.interval = time.Second

	// add reachable
	testAddHTTP(t, syncer)
	require.NoError(t, syncer.Start())

	// wait failed to synchronize
	require.NoError(t, syncer.Delete("http"))
	time.Sleep(3 * time.Second)
	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_synchronizePanic(t *testing.T) {
	dnsClient, pool, manager := testdns.DNSClient(t)
	defer func() { require.NoError(t, manager.Close()) }()

	syncer := New(pool, dnsClient, logger.Test)
	// add reachable
	testAddHTTP(t, syncer)

	// remove context
	syncer.Clients()["http"].client.(*HTTP).ctx = nil
	require.Error(t, syncer.Start())
	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

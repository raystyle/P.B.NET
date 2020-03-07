package timesync

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/random"
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
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)
	testAddClients(t, syncer)

	// check default sync interval
	require.Equal(t, defaultSyncInterval, syncer.GetSyncInterval())

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
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	// set random sleep
	require.Error(t, syncer.SetSleep(0, 0))
	require.Error(t, syncer.SetSleep(10, 0))
	require.NoError(t, syncer.SetSleep(3, 5))

	// no clients
	require.Equal(t, ErrNoClients, syncer.Start())

	t.Run("invalid config", func(t *testing.T) {
		err := syncer.Add("invalid config", &Client{
			Mode:   ModeNTP,
			Config: `network = "foo network"`,
		})
		require.NoError(t, err)
		require.Error(t, syncer.Start())

		require.NoError(t, syncer.Delete("invalid config"))
	})

	t.Run("all failed", func(t *testing.T) {
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
	})

	syncer.Stop()
	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Start_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	// set random sleep
	require.Error(t, syncer.SetSleep(0, 0))
	require.Error(t, syncer.SetSleep(10, 0))
	require.NoError(t, syncer.SetSleep(3, 5))

	go func() {
		time.Sleep(3 * time.Second)
		syncer.Stop()
	}()

	require.NoError(t, syncer.Add("unreachable", testUnreachableClient()))
	require.Error(t, syncer.Start())

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_StartWalker(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	syncer.StartWalker()
	now := syncer.Now()
	time.Sleep(2 * time.Second)
	require.False(t, syncer.Now().Equal(now))

	syncer.Stop()
	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Add_Delete(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)
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
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

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

	t.Run("failed", func(t *testing.T) {
		require.NoError(t, syncer.Delete("http"))
		err = syncer.Add("unreachable", testUnreachableClient())
		require.NoError(t, err)
		require.Error(t, syncer.Test(context.Background()))

		testAddHTTP(t, syncer)
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		require.Error(t, syncer.Test(ctx))
	})

	t.Run("panic", func(t *testing.T) {
		client := new(HTTP)
		patchFunc := func(_ interface{}) (time.Time, bool, error) {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(client, "Query", patchFunc)
		defer pg.Unpatch()

		require.Error(t, syncer.Test(context.Background()))
	})

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_synchronizeLoop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	// force set synchronize interval
	syncer.sleepFixed = 0
	syncer.sleepRandom = 0
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

func TestSyncer_workerPanic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	patchFunc := func(_ time.Duration) *time.Ticker {
		panic(monkey.Panic)
	}
	pg := monkey.Patch(time.NewTicker, patchFunc)
	defer pg.Unpatch()

	syncer.wg.Add(1)
	syncer.walker()
	pg.Unpatch()

	syncer.Stop()
	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_synchronizeLoopPanic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	patchFunc := func() *random.Rand {
		panic(monkey.Panic)
	}
	pg := monkey.Patch(random.New, patchFunc)
	defer pg.Unpatch()

	syncer.wg.Add(1)
	syncer.synchronizeLoop()
	pg.Unpatch()

	syncer.Stop()
	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_synchronizePanic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() { require.NoError(t, proxyMgr.Close()) }()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	// add reachable
	testAddHTTP(t, syncer)

	// remove context
	syncer.Clients()["http"].client.(*HTTP).ctx = nil
	require.Error(t, syncer.Start())

	syncer.Stop()
	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Parallel(t *testing.T) {
	syncer := New(nil, nil, nil, logger.Test)
	const (
		tag1 = "test-01"
		tag2 = "test-02"
	)

	t.Run("simple", func(t *testing.T) {
		add1 := func() {
			err := syncer.Add(tag1, &Client{
				Mode:     ModeNTP,
				SkipTest: true,
			})
			require.NoError(t, err)
		}
		add2 := func() {
			err := syncer.Add(tag2, &Client{
				Mode:     ModeNTP,
				SkipTest: true,
			})
			require.NoError(t, err)
		}
		testsuite.RunParallel(add1, add2)

		get1 := func() {
			clients := syncer.Clients()
			require.Equal(t, 2, len(clients))
		}
		get2 := func() {
			clients := syncer.Clients()
			require.Equal(t, 2, len(clients))
		}
		testsuite.RunParallel(get1, get2)

		delete1 := func() {
			err := syncer.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := syncer.Delete(tag2)
			require.NoError(t, err)
		}
		testsuite.RunParallel(delete1, delete2)
	})

	t.Run("mixed", func(t *testing.T) {
		add := func() {
			err := syncer.Add(tag1, &Client{
				Mode:     ModeNTP,
				SkipTest: true,
			})
			require.NoError(t, err)
		}
		get := func() {
			_ = syncer.Clients()
		}
		del := func() {
			_ = syncer.Delete(tag1)
		}
		testsuite.RunParallel(add, get, del)
	})

	t.Run("interval", func(t *testing.T) {
		set := func() {
			err := syncer.SetSyncInterval(time.Minute)
			require.NoError(t, err)
		}
		get := func() {
			_ = syncer.GetSyncInterval()
		}
		testsuite.RunParallel(set, get)
	})

	syncer.Stop()
	testsuite.IsDestroyed(t, syncer)
}

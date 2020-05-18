package timesync

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/monkey"
	"project/internal/patch/toml"
	"project/internal/random"
	"project/internal/testsuite"
	"project/internal/testsuite/testdns"
)

func testAddHTTP(t *testing.T, syncer *Syncer) {
	data, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	err = syncer.Add("http", &Client{
		Mode:   ModeHTTP,
		Config: string(data),
	})
	require.NoError(t, err)
}

func testAddNTP(t *testing.T, syncer *Syncer) {
	data, err := ioutil.ReadFile("testdata/ntp.toml")
	require.NoError(t, err)
	err = syncer.Add("ntp", &Client{
		Mode:   ModeNTP,
		Config: string(data),
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
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)
	testAddClients(t, syncer)

	t.Run("default sync interval", func(t *testing.T) {
		interval := syncer.GetSyncInterval()
		require.Equal(t, defaultSyncInterval, interval)
	})

	t.Run("set sync interval", func(t *testing.T) {
		const interval = 15 * time.Minute
		err := syncer.SetSyncInterval(interval)
		require.NoError(t, err)

		require.Equal(t, interval, syncer.GetSyncInterval())
	})

	t.Run("set invalid sync interval", func(t *testing.T) {
		err := syncer.SetSyncInterval(3 * time.Hour)
		require.Error(t, err)
	})

	err := syncer.Start()
	require.NoError(t, err)
	t.Log("now: ", syncer.Now().Local())

	// wait walker self-add
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
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	t.Run("set random sleep", func(t *testing.T) {
		err := syncer.SetSleep(0, 0)
		require.Error(t, err)

		err = syncer.SetSleep(10, 0)
		require.Error(t, err)

		err = syncer.SetSleep(3, 5)
		require.NoError(t, err)
	})

	t.Run("no clients", func(t *testing.T) {
		err := syncer.Start()
		require.Equal(t, ErrNoClients, err)
	})

	t.Run("invalid config", func(t *testing.T) {
		const tag = "invalid config"

		err := syncer.Add(tag, &Client{
			Mode:   ModeNTP,
			Config: `network = "foo network"`,
		})
		require.NoError(t, err)

		err = syncer.Start()
		require.Error(t, err)

		err = syncer.Delete(tag)
		require.NoError(t, err)
	})

	t.Run("all failed", func(t *testing.T) {
		go func() {
			// delete unreachable
			time.Sleep(time.Second)
			err := syncer.Delete("unreachable")
			require.NoError(t, err)

			// add reachable
			testAddHTTP(t, syncer)
		}()

		err := syncer.Add("unreachable", testUnreachableClient())
		require.NoError(t, err)

		err = syncer.Start()
		require.NoError(t, err)
	})

	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Start_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	// set random sleep
	err := syncer.SetSleep(0, 0)
	require.Error(t, err)

	err = syncer.SetSleep(10, 0)
	require.Error(t, err)

	err = syncer.SetSleep(3, 5)
	require.NoError(t, err)

	go func() {
		time.Sleep(3 * time.Second)
		syncer.Stop()
	}()

	err = syncer.Add("unreachable", testUnreachableClient())
	require.NoError(t, err)

	err = syncer.Start()
	require.Error(t, err)

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_StartWalker(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
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
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)
	testAddClients(t, syncer)

	t.Run("unknown mode", func(t *testing.T) {
		err := syncer.Add("foo mode", &Client{Mode: "foo mode"})
		require.Error(t, err)
	})

	t.Run("invalid config", func(t *testing.T) {
		err := syncer.Add("invalid config", &Client{
			Mode:   ModeNTP,
			Config: string([]byte{1, 2, 3, 4}),
		})
		require.Error(t, err)
	})

	t.Run("exist", func(t *testing.T) {
		err := syncer.Add("ntp", &Client{
			Mode: ModeNTP,
		})
		require.Error(t, err)
	})

	t.Run("delete", func(t *testing.T) {
		err := syncer.Delete("http")
		require.NoError(t, err)

		err = syncer.Delete("http")
		require.Error(t, err)
	})

	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Test(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	ctx := context.Background()

	// no clients
	t.Run("no clients", func(t *testing.T) {
		err := syncer.Test(ctx)
		require.Equal(t, ErrNoClients, err)
	})

	t.Run("passed", func(t *testing.T) {
		// add reachable
		testAddHTTP(t, syncer)

		// add skip
		err := syncer.Add("skip", &Client{
			Mode:     ModeNTP,
			SkipTest: true,
		})
		require.NoError(t, err)

		err = syncer.Test(ctx)
		require.NoError(t, err)
	})

	t.Run("failed", func(t *testing.T) {
		err := syncer.Delete("http")
		require.NoError(t, err)

		err = syncer.Add("unreachable", testUnreachableClient())
		require.NoError(t, err)

		err = syncer.Test(ctx)
		require.Error(t, err)

		testAddHTTP(t, syncer)
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err := syncer.Test(ctx)
		require.Error(t, err)
	})

	t.Run("panic", func(t *testing.T) {
		client := new(HTTP)
		patch := func(interface{}) (time.Time, bool, error) {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(client, "Query", patch)
		defer pg.Unpatch()

		err := syncer.Test(ctx)
		require.Error(t, err)
	})

	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_synchronizeLoop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	// force set synchronize interval
	syncer.sleepFixed = 0
	syncer.sleepRandom = 0
	syncer.interval = time.Second

	// add reachable
	testAddHTTP(t, syncer)
	err := syncer.Start()
	require.NoError(t, err)

	// wait failed to synchronize
	err = syncer.Delete("http")
	require.NoError(t, err)
	time.Sleep(3 * time.Second)

	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_workerPanic(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	patch := func(time.Duration) *time.Ticker {
		panic(monkey.Panic)
	}
	pg := monkey.Patch(time.NewTicker, patch)
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
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	patch := func() *random.Rand {
		panic(monkey.Panic)
	}
	pg := monkey.Patch(random.NewRand, patch)
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
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := New(certPool, proxyPool, dnsClient, logger.Test)

	// add reachable
	testAddHTTP(t, syncer)

	// remove context
	syncer.Clients()["http"].client.(*HTTP).ctx = nil
	err := syncer.Start()
	require.Error(t, err)

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
			require.Len(t, clients, 2)
		}
		get2 := func() {
			clients := syncer.Clients()
			require.Len(t, clients, 2)
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

func TestClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/client.toml")
	require.NoError(t, err)

	// check unnecessary field
	client := Client{}
	err = toml.Unmarshal(data, &client)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, client)

	testdata := []*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "ntp", actual: client.Mode},
		{expected: true, actual: client.SkipTest},
		{expected: "address = \"2.pool.ntp.org:123\"", actual: client.Config},
	}
	for _, td := range testdata {
		require.Equal(t, td.expected, td.actual)
	}
}

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

func TestSyncer_Add(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	syncer := NewSyncer(nil, nil, nil, nil)

	t.Run("ok", func(t *testing.T) {
		err := syncer.Add("test-http", &Client{
			Mode: ModeHTTP,
		})
		require.NoError(t, err)
		err = syncer.Add("test-ntp", &Client{
			Mode: ModeNTP,
		})
		require.NoError(t, err)
	})

	t.Run("unknown mode", func(t *testing.T) {
		err := syncer.Add("foo mode", &Client{
			Mode: "foo mode",
		})
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("invalid config", func(t *testing.T) {
		err := syncer.Add("invalid config", &Client{
			Mode:   ModeNTP,
			Config: string([]byte{1, 2, 3, 4}),
		})
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("exist", func(t *testing.T) {
		const tag = "exist"

		client := &Client{
			Mode: ModeNTP,
		}
		err := syncer.Add(tag, client)
		require.NoError(t, err)
		err = syncer.Add(tag, client)
		require.Error(t, err)
	})

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Delete(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	syncer := NewSyncer(nil, nil, nil, nil)

	t.Run("ok", func(t *testing.T) {
		const tag = "test"

		client := &Client{
			Mode: ModeNTP,
		}
		err := syncer.Add(tag, client)
		require.NoError(t, err)

		err = syncer.Delete(tag)
		require.NoError(t, err)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		err := syncer.Delete("foo tag")
		require.Error(t, err)
		t.Log(err)
	})

	testsuite.IsDestroyed(t, syncer)
}

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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)
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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

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

func TestSyncer_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

	syncer.StartWalker()
	now := syncer.Now()
	time.Sleep(2 * time.Second)
	require.False(t, syncer.Now().Equal(now))

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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

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
	syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

	// add reachable
	testAddHTTP(t, syncer)

	// remove context
	syncer.Clients()["http"].client.(*HTTP).ctx = nil
	err := syncer.Start()
	require.Error(t, err)

	syncer.Stop()

	testsuite.IsDestroyed(t, syncer)
}

func TestSyncer_Add_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	const (
		tag1 = "http"
		tag2 = "ntp"
	)
	client1 := &Client{
		Mode: ModeHTTP,
	}
	client2 := &Client{
		Mode: ModeNTP,
	}

	t.Run("part", func(t *testing.T) {
		syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

		add1 := func() {
			err := syncer.Add(tag1, client1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := syncer.Add(tag2, client2)
			require.NoError(t, err)
		}
		cleanup := func() {
			clients := syncer.Clients()
			require.Len(t, clients, 2)

			err := syncer.Delete(tag1)
			require.NoError(t, err)
			err = syncer.Delete(tag2)
			require.NoError(t, err)

			clients = syncer.Clients()
			require.Empty(t, clients)

			// reset Client.client
			client1.client = nil
			client2.client = nil
		}
		testsuite.RunParallel(100, nil, cleanup, add1, add2)

		testsuite.IsDestroyed(t, syncer)
	})

	t.Run("whole", func(t *testing.T) {
		var syncer *Syncer

		init := func() {
			syncer = NewSyncer(certPool, proxyPool, dnsClient, logger.Test)
		}
		add1 := func() {
			err := syncer.Add(tag1, client1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := syncer.Add(tag2, client2)
			require.NoError(t, err)
		}
		cleanup := func() {
			clients := syncer.Clients()
			require.Len(t, clients, 2)

			err := syncer.Delete(tag1)
			require.NoError(t, err)
			err = syncer.Delete(tag2)
			require.NoError(t, err)

			clients = syncer.Clients()
			require.Empty(t, clients)

			// reset Client.client
			client1.client = nil
			client2.client = nil
		}
		testsuite.RunParallel(100, init, cleanup, add1, add2)

		testsuite.IsDestroyed(t, syncer)
	})
}

func TestSyncer_Delete_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	const (
		tag1 = "http"
		tag2 = "ntp"
	)
	client1 := &Client{
		Mode: ModeHTTP,
	}
	client2 := &Client{
		Mode: ModeNTP,
	}

	t.Run("part", func(t *testing.T) {
		syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

		init := func() {
			err := syncer.Add(tag1, client1)
			require.NoError(t, err)
			err = syncer.Add(tag2, client2)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := syncer.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := syncer.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			clients := syncer.Clients()
			require.Empty(t, clients)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		testsuite.IsDestroyed(t, syncer)
	})

	t.Run("whole", func(t *testing.T) {
		var syncer *Syncer

		init := func() {
			syncer = NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

			err := syncer.Add(tag1, client1)
			require.NoError(t, err)
			err = syncer.Add(tag2, client2)
			require.NoError(t, err)
		}
		delete1 := func() {
			err := syncer.Delete(tag1)
			require.NoError(t, err)
		}
		delete2 := func() {
			err := syncer.Delete(tag2)
			require.NoError(t, err)
		}
		cleanup := func() {
			clients := syncer.Clients()
			require.Empty(t, clients)
		}
		testsuite.RunParallel(100, init, cleanup, delete1, delete2)

		testsuite.IsDestroyed(t, syncer)
	})
}

func TestSyncer_Clients_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	const (
		tag1 = "http"
		tag2 = "ntp"
	)
	client1 := &Client{
		Mode: ModeHTTP,
	}
	client2 := &Client{
		Mode: ModeNTP,
	}

	t.Run("part", func(t *testing.T) {
		syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

		err := syncer.Add(tag1, client1)
		require.NoError(t, err)
		err = syncer.Add(tag2, client2)
		require.NoError(t, err)

		clients := func() {
			clients := syncer.Clients()
			require.Len(t, clients, 2)
		}
		testsuite.RunParallel(100, nil, nil, clients, clients)

		testsuite.IsDestroyed(t, syncer)
	})

	t.Run("whole", func(t *testing.T) {
		var syncer *Syncer

		init := func() {
			syncer = NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

			err := syncer.Add(tag1, client1)
			require.NoError(t, err)
			err = syncer.Add(tag2, client2)
			require.NoError(t, err)
		}
		clients := func() {
			clients := syncer.Clients()
			require.Len(t, clients, 2)
		}
		cleanup := func() {
			err := syncer.Delete(tag1)
			require.NoError(t, err)
			err = syncer.Delete(tag2)
			require.NoError(t, err)

			clients := syncer.Clients()
			require.Empty(t, clients)
		}
		testsuite.RunParallel(100, init, cleanup, clients, clients)

		testsuite.IsDestroyed(t, syncer)
	})
}

func TestSyncer_SyncInterval_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("part", func(t *testing.T) {
		syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

		get := func() {
			_ = syncer.GetSyncInterval()
		}
		set := func() {
			const interval = 3 * time.Minute

			err := syncer.SetSyncInterval(interval)
			require.NoError(t, err)

			i := syncer.GetSyncInterval()
			require.Equal(t, interval, i)
		}
		testsuite.RunParallel(100, nil, nil, get, get, set, set)

		testsuite.IsDestroyed(t, syncer)
	})

	t.Run("whole", func(t *testing.T) {
		var syncer *Syncer

		init := func() {
			syncer = NewSyncer(certPool, proxyPool, dnsClient, logger.Test)
		}
		get := func() {
			_ = syncer.GetSyncInterval()
		}
		set := func() {
			const interval = 3 * time.Minute

			err := syncer.SetSyncInterval(interval)
			require.NoError(t, err)

			i := syncer.GetSyncInterval()
			require.Equal(t, interval, i)
		}
		testsuite.RunParallel(100, init, nil, get, get, set, set)

		testsuite.IsDestroyed(t, syncer)
	})
}

func TestSyncer_Synchronize_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	t.Run("part", func(t *testing.T) {
		syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

		testAddClients(t, syncer)
		err := syncer.Start()
		require.NoError(t, err)

		synchronize := func() {
			err := syncer.Synchronize()
			require.NoError(t, err)
			t.Log(syncer.Now())
		}
		testsuite.RunParallel(2, nil, nil, synchronize, synchronize)

		syncer.Stop()

		testsuite.IsDestroyed(t, syncer)
	})

	t.Run("whole", func(t *testing.T) {
		var syncer *Syncer

		init := func() {
			syncer = NewSyncer(certPool, proxyPool, dnsClient, logger.Test)

			testAddClients(t, syncer)
			err := syncer.Start()
			require.NoError(t, err)
		}
		synchronize := func() {
			err := syncer.Synchronize()
			require.NoError(t, err)
			t.Log(syncer.Now())
		}
		cleanup := func() {
			syncer.Stop()
		}
		testsuite.RunParallel(2, init, cleanup, synchronize, synchronize)

		testsuite.IsDestroyed(t, syncer)
	})
}

func TestSyncer_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	dnsClient, proxyPool, proxyMgr, certPool := testdns.DNSClient(t)
	defer func() {
		err := proxyMgr.Close()
		require.NoError(t, err)
	}()

	const (
		tag1 = "test-http-1"
		tag2 = "test-ntp-1"
		tag3 = "test-http-2"
		tag4 = "test-ntp-2"
	)

	httpClient, err := ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	ntpClient, err := ioutil.ReadFile("testdata/ntp.toml")
	require.NoError(t, err)
	client1 := &Client{
		Mode:   ModeHTTP,
		Config: string(httpClient),
	}
	client2 := &Client{
		Mode:   ModeNTP,
		Config: string(ntpClient),
	}
	client3 := &Client{
		Mode:   ModeHTTP,
		Config: string(httpClient),
	}
	client4 := &Client{
		Mode:   ModeNTP,
		Config: string(ntpClient),
	}

	t.Run("without stop", func(t *testing.T) {
		t.Run("part", func(t *testing.T) {
			syncer := NewSyncer(certPool, proxyPool, dnsClient, logger.Test)
			testAddClients(t, syncer)

			init := func() {
				err := syncer.Add(tag1, client1)
				require.NoError(t, err)
				err = syncer.Add(tag2, client2)
				require.NoError(t, err)

				err = syncer.Start()
				require.NoError(t, err)
			}
			add1 := func() {
				err := syncer.Add(tag3, client3)
				require.NoError(t, err)
			}
			add2 := func() {
				err := syncer.Add(tag4, client4)
				require.NoError(t, err)
			}
			delete1 := func() {
				err := syncer.Delete(tag1)
				require.NoError(t, err)
			}
			delete2 := func() {
				err := syncer.Delete(tag2)
				require.NoError(t, err)
			}
			clients := func() {
				clients := syncer.Clients()
				require.NotEmpty(t, clients)
			}
			getSyncInterval := func() {
				_ = syncer.GetSyncInterval()
			}
			setSyncInterval := func() {
				const interval = 3 * time.Minute

				err := syncer.SetSyncInterval(interval)
				require.NoError(t, err)

				i := syncer.GetSyncInterval()
				require.Equal(t, interval, i)
			}
			now1 := func() {
				t.Log("now1:", syncer.Now())
			}
			now2 := func() {
				t.Log("now2:", syncer.Now())
			}
			synchronize := func() {
				err := syncer.Synchronize()
				require.NoError(t, err)
			}
			cleanup := func() {
				err := syncer.Delete(tag3)
				require.NoError(t, err)
				err = syncer.Delete(tag4)
				require.NoError(t, err)

				// reset Client.client
				client1.client = nil
				client2.client = nil
				client3.client = nil
				client4.client = nil
			}
			fns := []func(){
				add1, add2, delete1, delete2, clients,
				getSyncInterval, setSyncInterval,
				now1, now2, synchronize, synchronize,
			}
			testsuite.RunParallel(2, init, cleanup, fns...)

			syncer.Stop()

			testsuite.IsDestroyed(t, syncer)
		})

		t.Run("whole", func(t *testing.T) {

		})
	})

	t.Run("with stop", func(t *testing.T) {

	})
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

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "ntp", actual: client.Mode},
		{expected: true, actual: client.SkipTest},
		{expected: "address = \"2.pool.ntp.org:123\"", actual: client.Config},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

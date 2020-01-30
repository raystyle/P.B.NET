package controller

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"

	"project/testdata"
)

var (
	ctrl     *CTRL
	initOnce sync.Once
)

func TestMain(m *testing.M) {
	m.Run()

	// wait to print log
	if ctrl != nil {
		time.Sleep(time.Second)
		ctrl.Exit(nil)
	}

	testdata.Clean()

	// one test main goroutine and
	// two goroutine about pprof server in testsuite
	leaks := true
	for i := 0; i < 300; i++ {
		if runtime.NumGoroutine() == 3 {
			leaks = false
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if leaks {
		fmt.Println("[warning] goroutine leaks!")
		os.Exit(1)
	}

	if ctrl != nil {
		ctrlC := ctrl // must copy, global variable
		ctrl = nil

		if !testsuite.Destroyed(ctrlC) {
			fmt.Println("[warning] controller is not destroyed")
			os.Exit(1)
		}
	}
}

func testInitializeController(t testing.TB) {
	initOnce.Do(func() {
		err := os.Chdir("../app")
		require.NoError(t, err)
		cfg := testGenerateConfig()
		ctrl, err = New(cfg)
		require.NoError(t, err)
		_, err = ctrl.database.SelectBoot()
		if err != nil {
			err = InitializeDatabase(cfg)
			require.NoError(t, err)
			// add test data
			testInsertProxyClient(t)
			testInsertDNSServer(t)
			testInsertTimeSyncerClient(t)
			testInsertBoot(t)
			testInsertListener(t)
		}
		testsuite.IsDestroyed(t, cfg)
		// set controller keys
		err = ctrl.LoadSessionKeyFromFile("key/session.key", []byte("pbnet"))
		require.NoError(t, err)
		go func() {
			err := ctrl.Main()
			require.NoError(t, err)
		}()
		ctrl.Wait()
	})
}

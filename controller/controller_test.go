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

	testdata.Clean()

	// wait to print log
	time.Sleep(time.Second)
	ctrl.Exit(nil)

	// one test main goroutine and
	// two goroutine about pprof server in testsuite
	if runtime.NumGoroutine() != 3 {
		fmt.Println("[Warning] goroutine leaks!")
		os.Exit(1)
	}

	// must copy
	ctrlC := ctrl
	ctrl = nil
	if !testsuite.Destroyed(ctrlC) {
		fmt.Println("[Warning] controller is not destroyed")
		os.Exit(1)
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

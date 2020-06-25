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
	ctrl     *Ctrl
	initOnce sync.Once
)

func TestMain(m *testing.M) {
	exitCode := m.Run()
	if ctrl != nil {
		// wait to print log
		time.Sleep(250 * time.Millisecond)
		ctrl.Exit(nil)
	}
	testdata.Clean()
	// one test main goroutine and two goroutine about
	// pprof server in internal/testsuite.go
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
		time.Sleep(time.Minute)
		os.Exit(1)
	}
	if ctrl != nil {
		// must copy, because it is a global variable
		ctrlC := ctrl
		ctrl = nil
		if !testsuite.Destroyed(ctrlC) {
			fmt.Println("[warning] controller is not destroyed")
			time.Sleep(time.Minute)
			os.Exit(1)
		}
	}
	os.Exit(exitCode)
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
			testInsertZone(t)
		}
		testsuite.IsDestroyed(t, cfg)
		// set controller keys
		err = ctrl.LoadKeyFromFile([]byte("admin"), []byte("cert"))
		require.NoError(t, err)
		go func() {
			err := ctrl.Main()
			require.NoError(t, err)
		}()
		ctrl.Wait()
	})
}

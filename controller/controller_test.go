package controller

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

var (
	ctrl     *CTRL
	initOnce sync.Once
)

func testInitCtrl(t testing.TB) {
	initOnce.Do(func() {
		err := os.Chdir("../app")
		require.NoError(t, err)
		cfg := testGenerateConfig()
		ctrl, err = New(cfg)
		require.NoError(t, err)
		_, err = ctrl.db.SelectBoot()
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
		err = ctrl.LoadSessionKey([]byte("pbnet"))
		require.NoError(t, err)
		go func() {
			err := ctrl.Main()
			require.NoError(t, err)
		}()
		ctrl.Wait()
	})
}

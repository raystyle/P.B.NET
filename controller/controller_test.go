package controller

import (
	"os"
	"sync"

	"github.com/stretchr/testify/require"
)

func init() {
	err := os.Chdir("../app")
	if err != nil {
		panic(err)
	}
}

var (
	ctrl     *CTRL
	initOnce sync.Once
)

func testInitCtrl(t require.TestingT) {
	initOnce.Do(func() {
		cfg := testGenerateConfig()
		controller, err := New(cfg)
		if err != nil {
			// initialize database
			err = InitializeDatabase(cfg)
			require.NoError(t, err)
			// add test data
			db, err := newDB(cfg)
			require.NoError(t, err)
			ctrl = &CTRL{db: db}
			testInsertProxyClient(t)
			testInsertDNSServer(t)
			testInsertTimeSyncerClient(t)
			testInsertBoot(t)
			testInsertListener(t)
			ctrl, err = New(cfg)
			require.NoError(t, err)
		} else {
			ctrl = controller
		}

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

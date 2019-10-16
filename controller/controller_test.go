package controller

import (
	"net/http"
	_ "net/http/pprof"
	"sync"

	"github.com/stretchr/testify/require"

	"project/testdata"
)

var (
	ctrl     *CTRL
	initOnce sync.Once
)

func init() {
	go func() { _ = http.ListenAndServe("localhost:8080", nil) }()
}

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
			testInsertTimeSyncerConfig(t)
			testInsertBoot(t)
			testInsertListener(t)
			ctrl, err = New(cfg)
			require.NoError(t, err)
		} else {
			ctrl = controller
		}
		err = ctrl.LoadKeys(testdata.CtrlKeysPWD)
		require.NoError(t, err)
		go func() {
			err := ctrl.Main()
			require.NoError(t, err)
		}()
		ctrl.TestWait()
	})
}

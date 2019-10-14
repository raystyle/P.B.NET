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

func testInitCtrl(t require.TestingT) {
	initOnce.Do(func() {
		cfg := testGenerateConfig()
		controller, err := New(cfg)
		if err != nil {
			// init database
			err = InitializeDatabase(cfg)
			require.NoError(t, err)
			// add test data
			ctrl = new(CTRL)
			db, err := newDB(ctrl, cfg)
			require.NoError(t, err)
			ctrl.db = db
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

func pprof() {
	go func() { _ = http.ListenAndServe("localhost:8080", nil) }()
}

/*
func Test_gorm(t *testing.T) {
	c := testGenerateConfig()
	db, err := gorm.Open(c.Dialect, c.DSN)
	require.NoError(t, err)
	db.LogMode(true)
	db.SingularTable(true) // not add s
}
*/

package controller

import (
	"net/http"
	_ "net/http/pprof"
	"sync"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"

	"project/testdata"
)

var (
	ctrl     *CTRL
	initOnce sync.Once
)

func initCtrl(t require.TestingT) {
	initOnce.Do(func() {
		cfg := testGenerateConfig()
		controller, err := New(cfg)
		if err != nil {
			// init database
			err = InitDatabase(cfg)
			require.NoError(t, err)
			// add test data
			// connect database
			db, err := gorm.Open(cfg.Dialect, cfg.DSN)
			require.NoError(t, err)
			db.SingularTable(true) // not add s
			ctrl = &CTRL{db: db}
			testInsertProxyClient(t)
			testInsertDNSServer(t)
			testInsertTimeSyncerConfig(t)
			testInsertBoot(t)
			testInsertListener(t)
			_ = db.Close()
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
		ctrl.Wait()
	})
}

func pprof() {
	go func() { _ = http.ListenAndServe(":8080", nil) }()
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

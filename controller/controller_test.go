package controller

import (
	"sync"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"

	"project/testdata"
)

var (
	ctrl      *CTRL
	once_init sync.Once
)

func init_ctrl(t *testing.T) {
	once_init.Do(func() {
		c := test_gen_config()
		controller, err := New(c)
		if err != nil {
			// init database
			err = Init_Database(c)
			require.Nil(t, err, err)
			// add test data
			// connect database
			db, err := gorm.Open(c.Dialect, c.DSN)
			require.Nil(t, err, err)
			db.SingularTable(true) // not add s
			ctrl = &CTRL{db: db}
			test_insert_proxy_client(t)
			test_insert_dns_client(t)
			test_insert_timesync(t)
			test_insert_boot(t)
			test_insert_listener(t)
			_ = db.Close()
			ctrl, err = New(c)
			require.Nil(t, err, err)
		} else {
			ctrl = controller
		}
		err = ctrl.Load_Keys(testdata.CTRL_Keys_PWD)
		require.Nil(t, err, err)
		go func() {
			err := ctrl.Main()
			require.Nil(t, err, err)
		}()
	})
}

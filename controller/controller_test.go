package controller

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/testdata"
)

var (
	ctrl      *CTRL
	once_init sync.Once
)

func init_ctrl(t *testing.T) {
	once_init.Do(func() {
		config := test_gen_config()
		c, err := New(config)
		if err != nil {
			// init database
			config.Init_DB = true
			c, err = New(config)
			require.Nil(t, err, err)
			err = c.Init_Database()
			require.Nil(t, err, err)
			c.Exit()
			config.Init_DB = false
			ctrl, err = New(config)
			require.Nil(t, err, err)
		} else {
			ctrl = c
		}
		err = ctrl.Load_Keys(testdata.CTRL_Keys_PWD)
		require.Nil(t, err, err)
		go func() {
			err := ctrl.Main()
			require.Nil(t, err, err)
		}()
	})
}

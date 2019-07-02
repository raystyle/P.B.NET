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
		controller, err := New(config)
		if err != nil {
			// init database
			err = Init_Database(config)
			require.Nil(t, err, err)
			ctrl, err = New(config)
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

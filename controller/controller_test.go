package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CTRL(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	err := ctrl.Main()
	require.Nil(t, err, err)
}

func test_gen_ctrl(t *testing.T) *CTRL {
	ctrl, err := New(test_gen_config())
	require.Nil(t, err, err)
	err = ctrl.global.Load_Keys("123456789012")
	require.Nil(t, err, err)
	return ctrl
}

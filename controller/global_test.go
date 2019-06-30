package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_new_global(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	global, err := new_global(ctrl, test_gen_config())
	require.Nil(t, err, err)
	t.Log("now: ", global.Now())
}

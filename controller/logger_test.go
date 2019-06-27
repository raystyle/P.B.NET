package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func Test_CTRL_Logger(t *testing.T) {
	CTRL, err := New(test_gen_config())
	require.Nil(t, err, err)
	CTRL.Printf(logger.DEBUG, "test src", "test format %s", "test log")
	CTRL.Print(logger.DEBUG, "test src", "test print", "test log")
	CTRL.Println(logger.DEBUG, "test src", "test println", "test log")
}

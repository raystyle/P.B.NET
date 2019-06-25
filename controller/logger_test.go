package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func Test_ctrl_logger(t *testing.T) {
	CTRL, err := New(test_gen_config())
	require.Nil(t, err, err)
	db, err := new_database(CTRL)
	require.Nil(t, err, err)
	err = db.Connect()
	require.Nil(t, err, err)
	CTRL.database = db
	ctrl_l, err := new_ctrl_logger(CTRL)
	require.Nil(t, err, err)
	ctrl_l.Printf(logger.DEBUG, "test src", "test format %s", "test log")
	ctrl_l.Print(logger.DEBUG, "test src", "test print", "test log")
	ctrl_l.Println(logger.DEBUG, "test src", "test println", "test log")
}

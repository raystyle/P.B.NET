package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func Test_db_logger(t *testing.T) {
	path := os.TempDir() + "/database.log"
	l, err := new_db_logger("mysql", path)
	require.Nil(t, err, err)
	l.Print("test db log")
	l.Close()
	err = os.Remove(path)
	require.Nil(t, err, err)
}

func Test_gorm_logger(t *testing.T) {
	path := os.TempDir() + "/gorm.log"
	l, err := new_gorm_logger(path)
	require.Nil(t, err, err)
	l.Print("test gorm log")
	l.Close()
	err = os.Remove(path)
	require.Nil(t, err, err)
}

func Test_ctrl_logger(t *testing.T) {
	init_ctrl(t)
	ctrl.Printf(logger.DEBUG, "test src", "test format %s", "test log")
	ctrl.Print(logger.DEBUG, "test src", "test print", "test log")
	ctrl.Println(logger.DEBUG, "test src", "test println", "test log")
}

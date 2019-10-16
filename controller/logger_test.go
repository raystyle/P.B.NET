package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func TestDBLogger(t *testing.T) {
	path := os.TempDir() + "/database.log"
	l, err := newDBLogger("mysql", path)
	require.NoError(t, err)
	l.Print("test db log")
	l.Close()
	err = os.Remove(path)
	require.NoError(t, err)
}

func TestGormLogger(t *testing.T) {
	path := os.TempDir() + "/gorm.log"
	l, err := newGormLogger(path)
	require.NoError(t, err)
	l.Print("test gorm log")
	l.Close()
	err = os.Remove(path)
	require.NoError(t, err)
}

func TestCtrlLogger(t *testing.T) {
	testInitCtrl(t)
	ctrl.logger.Printf(logger.Debug, "test src", "test format %s", "test log")
	ctrl.logger.Print(logger.Debug, "test src", "test print", "test log")
	ctrl.logger.Println(logger.Debug, "test src", "test println", "test log")
}

package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func TestDatabaseLogger(t *testing.T) {
	path := os.TempDir() + "/database.log"
	l, err := newDatabaseLogger("mysql", path, os.Stdout)
	require.NoError(t, err)
	l.Print("test", "database", "log")
	l.Close()
	err = os.Remove(path)
	require.NoError(t, err)
}

func TestGormLogger(t *testing.T) {
	path := os.TempDir() + "/gorm.log"
	l, err := newGormLogger(path, os.Stdout)
	require.NoError(t, err)
	l.Print("test", "gorm", "log")
	l.Close()
	err = os.Remove(path)
	require.NoError(t, err)
}

func TestLogger(t *testing.T) {
	testInitializeController(t)
	const (
		testSrc  = "test src"
		testLog1 = "test"
		testLog2 = "log"
	)
	ctrl.logger.Printf(logger.Debug, testSrc, "test format %s %s", testLog1, testLog2)
	ctrl.logger.Print(logger.Debug, testSrc, "test print", testLog1, testLog2)
	ctrl.logger.Println(logger.Debug, testSrc, "test println", testLog1, testLog2)
}

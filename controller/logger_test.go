package controller

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func TestDatabaseLogger(t *testing.T) {
	testInitializeController(t)

	path := os.TempDir() + "/database.log"
	l, err := newDatabaseLogger(ctrl, "mysql", path, os.Stdout)
	require.NoError(t, err)
	l.Print("test", "database", "log")
	l.Close()

	err = os.Remove(path)
	require.NoError(t, err)
}

func TestGormLogger(t *testing.T) {
	testInitializeController(t)

	path := os.TempDir() + "/gorm.log"
	l, err := newGormLogger(ctrl, path, os.Stdout)
	require.NoError(t, err)
	l.Print("test", "gorm", "log")
	l.Close()

	err = os.Remove(path)
	require.NoError(t, err)
}

func TestLogger(t *testing.T) {
	testInitializeController(t)

	const (
		prefixF  = "test format %s %s"
		prefix   = "test print"
		prefixLn = "test println"
		src      = "test src"
		log1     = "test"
		log2     = "log"
	)

	lg := ctrl.logger

	lg.Printf(logger.Debug, src, prefixF, log1, log2)
	lg.Print(logger.Debug, src, prefix, log1, log2)
	lg.Println(logger.Debug, src, prefixLn, log1, log2)

	lg.Printf(logger.Warning, src, prefixF, log1, log2)
	lg.Print(logger.Warning, src, prefix, log1, log2)
	lg.Println(logger.Warning, src, prefixLn, log1, log2)

	lg.Printf(logger.Exploit, src, prefixF, log1, log2)
	lg.Print(logger.Exploit, src, prefix, log1, log2)
	lg.Println(logger.Exploit, src, prefixLn, log1, log2)

	lg.Printf(logger.Fatal, src, prefixF, log1, log2)
	lg.Print(logger.Fatal, src, prefix, log1, log2)
	lg.Println(logger.Fatal, src, prefixLn, log1, log2)
}

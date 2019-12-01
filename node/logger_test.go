package node

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func TestLogger(t *testing.T) {
	const (
		testSrc  = "test src"
		testLog1 = "test"
		testLog2 = "log"
	)
	node, err := New(testGenerateConfig(t))
	require.NoError(t, err)
	node.logger.Printf(logger.Debug, testSrc, "test format %s %s", testLog1, testLog2)
	node.logger.Print(logger.Debug, testSrc, "test print", testLog1, testLog2)
	node.logger.Println(logger.Debug, testSrc, "test println", testLog1, testLog2)
}

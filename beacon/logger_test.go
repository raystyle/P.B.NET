package beacon

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
	beacon, err := New(testGenerateConfig(t))
	require.NoError(t, err)
	beacon.logger.Printf(logger.Debug, testSrc, "test format %s %s", testLog1, testLog2)
	beacon.logger.Print(logger.Debug, testSrc, "test print", testLog1, testLog2)
	beacon.logger.Println(logger.Debug, testSrc, "test println", testLog1, testLog2)
}

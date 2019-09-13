package node

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func TestLogger(t *testing.T) {
	node, err := New(testGenerateConfig(t, true))
	require.NoError(t, err)
	node.Printf(logger.Debug, "test src", "test format %s", "test log")
	node.Print(logger.Debug, "test src", "test print", "test log")
	node.Println(logger.Debug, "test src", "test println", "test log")
}

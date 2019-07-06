package node

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func Test_Logger(t *testing.T) {
	node, err := New(test_gen_config(t, true))
	require.Nil(t, err, err)
	node.Printf(logger.DEBUG, "test src", "test format %s", "test log")
	node.Print(logger.DEBUG, "test src", "test print", "test log")
	node.Println(logger.DEBUG, "test src", "test println", "test log")
}

package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestRequest_SetID(t *testing.T) {
	test := new(TestRequest)
	test.SetID(1)
	require.Equal(t, uint64(1), test.ID)
}

func TestPluginRequest_SetID(t *testing.T) {
	plugin := new(PluginRequest)
	plugin.SetID(1)
	require.Equal(t, uint64(1), plugin.ID)
}

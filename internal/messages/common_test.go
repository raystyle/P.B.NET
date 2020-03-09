package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestRequest_SetID(t *testing.T) {
	test := new(TestRequest)
	g := testGenerateGUID()
	test.SetID(g)
	require.Equal(t, *g, test.ID)
}

func TestPluginRequest_SetID(t *testing.T) {
	plugin := new(PluginRequest)
	g := testGenerateGUID()
	plugin.SetID(g)
	require.Equal(t, *g, plugin.ID)
}

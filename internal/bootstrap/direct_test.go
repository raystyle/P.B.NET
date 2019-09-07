package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirect(t *testing.T) {
	nodes := testGenerateNodes()
	direct := NewDirect(nodes)
	_ = direct.Validate()
	b, err := direct.Marshal()
	require.NoError(t, err)
	direct = NewDirect(nil)
	err = direct.Unmarshal(b)
	require.NoError(t, err)
	resolved, _ := direct.Resolve()
	require.Equal(t, nodes, resolved)
}

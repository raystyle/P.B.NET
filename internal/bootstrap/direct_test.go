package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestDirect(t *testing.T) {
	nodes := testGenerateNodes()
	direct := NewDirect(nodes)
	_ = direct.Validate()
	b, err := direct.Marshal()
	require.NoError(t, err)
	testsuite.IsDestroyed(t, direct)

	direct = NewDirect(nil)
	err = direct.Unmarshal(b)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		resolved, _ := direct.Resolve()
		require.Equal(t, nodes, resolved)
	}
	testsuite.IsDestroyed(t, direct)
}

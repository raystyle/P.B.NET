package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Direct(t *testing.T) {
	nodes := test_generate_bootstrap_node()
	direct := New_Direct(nodes)
	_, _ = direct.Generate(nil)
	b, err := direct.Marshal()
	require.Nil(t, err, err)
	err = direct.Unmarshal(b)
	require.Nil(t, err, err)
	resolve, _ := direct.Resolve()
	require.Equal(t, nodes, resolve)
}

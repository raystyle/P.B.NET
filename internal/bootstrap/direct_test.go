package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Direct(t *testing.T) {
	nodes := test_generate_nodes()
	direct := New_Direct(nodes)
	_ = direct.Validate()
	b, err := direct.Marshal()
	require.Nil(t, err, err)
	direct = New_Direct(nil)
	err = direct.Unmarshal(b)
	require.Nil(t, err, err)
	resolved, _ := direct.Resolve()
	require.Equal(t, nodes, resolved)
}

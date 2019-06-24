package node

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Genesis_Node(t *testing.T) {
	test_node(t, true)
}

func Test_Common_Node(t *testing.T) {
	test_node(t, false)
}

func test_node(t *testing.T, genesis bool) {
	c := test_gen_config(t, genesis)
	node, err := New(c)
	require.Nil(t, err, err)
	err = node.Main()
	require.Nil(t, err, err)
}

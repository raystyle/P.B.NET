package node

import (
	"net/http"
	_ "net/http/pprof"
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
	node, err := New(test_gen_config(t, genesis))
	require.Nil(t, err, err)
	err = node.Main()
	require.Nil(t, err, err)
}

func pprof() {
	go func() { _ = http.ListenAndServe(":8080", nil) }()
}

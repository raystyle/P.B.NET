package node

import (
	"net/http"
	_ "net/http/pprof"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenesisNode(t *testing.T) {
	testNode(t, true)
}

func TestCommonNode(t *testing.T) {
	testNode(t, false)
}

func testNode(t *testing.T, genesis bool) {
	node, err := New(testGenerateConfig(t, genesis))
	require.NoError(t, err)
	err = node.Main()
	require.NoError(t, err)
}

func pprof() {
	go func() { _ = http.ListenAndServe("localhost:8080", nil) }()
}

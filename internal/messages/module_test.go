package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSingleShell_SetID(t *testing.T) {
	ss := new(SingleShell)
	g := testGenerateGUID()
	ss.SetID(g)
	require.Equal(t, *g, ss.ID)
}

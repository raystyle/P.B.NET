package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellCode_SetID(t *testing.T) {
	esc := new(ShellCode)
	g := testGenerateGUID()
	esc.SetID(g)
	require.Equal(t, *g, esc.ID)
}

func TestSingleShell_SetID(t *testing.T) {
	ss := new(SingleShell)
	g := testGenerateGUID()
	ss.SetID(g)
	require.Equal(t, *g, ss.ID)
}

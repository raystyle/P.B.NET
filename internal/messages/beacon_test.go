package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeMode_SetID(t *testing.T) {
	cm := new(ChangeMode)
	g := testGenerateGUID()
	cm.SetID(g)
	require.Equal(t, *g, cm.ID)
}

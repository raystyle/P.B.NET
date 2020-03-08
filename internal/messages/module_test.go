package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSingleShell_SetID(t *testing.T) {
	ss := new(SingleShell)
	ss.SetID(1)
	require.Equal(t, uint64(1), ss.ID)
}

package messages

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeMode_SetID(t *testing.T) {
	cm := new(ChangeMode)
	cm.SetID(1)
	require.Equal(t, uint64(1), cm.ID)
}

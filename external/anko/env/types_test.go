package env

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnv_Types(t *testing.T) {
	type Foo struct {
		A string
	}

	env := NewEnv()
	err := env.DefineType("test", Foo{})
	require.NoError(t, err)

	types := env.Types()
	typ, ok := types["test"]
	require.True(t, ok)
	require.Equal(t, "env.Foo", typ.String())
}

package env

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnv_Values(t *testing.T) {
	env := NewEnv()
	err := env.Define("test", "test str")
	require.NoError(t, err)

	values := env.Values()
	v, ok := values["test"]
	require.True(t, ok)
	require.Equal(t, "test str", v.Interface().(string))
}

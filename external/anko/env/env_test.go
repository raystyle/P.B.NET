package env

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	env := NewEnv()
	err := env.Define("a", "a")
	require.NoError(t, err)
	output := env.String()
	expected := `No parent
a = "a"
`
	require.Equal(t, expected, output)

	env = env.NewEnv()
	err = env.Define("b", "b")
	require.NoError(t, err)
	output = env.String()
	expected = `Has parent
b = "b"
`
	require.Equal(t, expected, output)

	env = NewEnv()
	err = env.Define("c", "c")
	require.NoError(t, err)
	err = env.DefineType("string", "a")
	require.NoError(t, err)
	output = env.String()
	expected = `No parent
c = "c"
string = string
`
	require.Equal(t, expected, output)
}

func TestGetEnvFromPath(t *testing.T) {

}

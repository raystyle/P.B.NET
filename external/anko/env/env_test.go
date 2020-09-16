package env

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnv_String(t *testing.T) {
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

func TestEnv_GetEnvFromPath(t *testing.T) {
	env := NewEnv()

	a, err := env.NewModule("a")
	require.NoError(t, err)

	b, err := a.NewModule("b")
	require.NoError(t, err)

	c, err := b.NewModule("c")
	require.NoError(t, err)

	err = c.Define("d", "d")
	require.NoError(t, err)

	e, err := env.GetEnvFromPath(nil)
	require.NoError(t, err)
	require.NotNil(t, e)

	e, err = env.GetEnvFromPath([]string{})
	require.NoError(t, err)
	require.NotNil(t, e)

	t.Run("a.b.c", func(t *testing.T) {
		e, err = env.GetEnvFromPath([]string{"a", "c"})
		require.EqualError(t, err, "no namespace called: c")
		require.Nil(t, e)

		e, err = env.GetEnvFromPath([]string{"a", "b", "c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err := e.Get("d")
		require.NoError(t, err)
		v, ok := value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)

		e, err = a.GetEnvFromPath([]string{"a", "b", "c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err = e.Get("d")
		require.NoError(t, err)
		v, ok = value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)

		e, err = b.GetEnvFromPath([]string{"a", "b", "c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err = e.Get("d")
		require.NoError(t, err)
		v, ok = value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)

		e, err = c.GetEnvFromPath([]string{"a", "b", "c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err = e.Get("d")
		require.NoError(t, err)
		v, ok = value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)
	})

	t.Run("b.c", func(t *testing.T) {
		e, err = env.GetEnvFromPath([]string{"b", "c"})
		require.EqualError(t, err, "no namespace called: b")
		require.Nil(t, e)

		e, err = a.GetEnvFromPath([]string{"b", "c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err := e.Get("d")
		require.NoError(t, err)
		v, ok := value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)

		e, err = b.GetEnvFromPath([]string{"b", "c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err = e.Get("d")
		require.NoError(t, err)
		v, ok = value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)

		e, err = c.GetEnvFromPath([]string{"b", "c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err = e.Get("d")
		require.NoError(t, err)
		v, ok = value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)
	})

	t.Run("c", func(t *testing.T) {
		e, err = env.GetEnvFromPath([]string{"c"})
		require.EqualError(t, err, "no namespace called: c")
		require.Nil(t, e)

		e, err = b.GetEnvFromPath([]string{"c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err := e.Get("d")
		require.NoError(t, err)
		v, ok := value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)

		e, err = c.GetEnvFromPath([]string{"c"})
		require.NoError(t, err)
		require.NotNil(t, e)
		value, err = e.Get("d")
		require.NoError(t, err)
		v, ok = value.(string)
		require.True(t, ok)
		require.Equal(t, "d", v)
	})
}

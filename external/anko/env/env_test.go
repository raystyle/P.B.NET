package env

import (
	"reflect"
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

func TestEnv_Copy(t *testing.T) {
	parent := NewEnv()
	err := parent.Define("a", "a")
	require.NoError(t, err)
	err = parent.DefineType("b", []bool{})
	require.NoError(t, err)

	child := parent.NewEnv()
	err = child.Define("c", "c")
	require.NoError(t, err)
	err = child.DefineType("d", []int64{})
	require.NoError(t, err)

	copied := child.Copy()

	// get and type
	val, err := copied.Get("a")
	require.NoError(t, err)
	require.Equal(t, "a", val)

	typ, err := copied.Type("b")
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf([]bool{}), typ)

	val, err = copied.Get("c")
	require.NoError(t, err)
	require.Equal(t, "c", val)

	typ, err = copied.Type("d")
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf([]int64{}), typ)

	// set and define
	err = copied.Set("a", "i")
	require.NoError(t, err)
	val, err = child.Get("a")
	require.NoError(t, err)
	require.Equal(t, "i", val, "parent was not modified")
	val, err = copied.Get("a")
	require.NoError(t, err)
	require.Equal(t, "i", val, "copied did not get parent value")

	err = copied.Set("c", "j")
	require.NoError(t, err)
	val, err = child.Get("c")
	require.NoError(t, err)
	require.Equal(t, "c", val, "parent was not modified")
	val, err = copied.Get("c")
	require.NoError(t, err)
	require.Equal(t, "j", val, "copied did not get parent value")

	err = child.Set("a", "x")
	require.NoError(t, err)
	val, err = child.Get("a")
	require.NoError(t, err)
	require.Equal(t, "x", val, "parent was not modified")
	val, err = copied.Get("a")
	require.NoError(t, err)
	require.Equal(t, "x", val, "copied did not get parent value")

	err = child.Set("c", "z")
	require.NoError(t, err)
	val, err = child.Get("c")
	require.NoError(t, err)
	require.Equal(t, "z", val, "parent was not modified")
	val, err = copied.Get("c")
	require.NoError(t, err)
	require.Equal(t, "j", val, "copied did not get parent value")

	err = parent.Set("a", "m")
	require.NoError(t, err)
	val, err = child.Get("a")
	require.NoError(t, err)
	require.Equal(t, "m", val, "parent was not modified")
	val, err = copied.Get("a")
	require.NoError(t, err)
	require.Equal(t, "m", val, "copied did not get parent value")

	err = parent.Define("x", "n")
	require.NoError(t, err)
	val, err = child.Get("x")
	require.NoError(t, err)
	require.Equal(t, "n", val, "parent was not modified")
	val, err = copied.Get("x")
	require.NoError(t, err)
	require.Equal(t, "n", val, "copied did not get parent value")
}

func TestEnv_DeepCopy(t *testing.T) {
	parent := NewEnv()
	err := parent.Define("a", "a")
	require.NoError(t, err)

	env := parent.NewEnv()
	copied := env.DeepCopy()

	val, err := copied.Get("a")
	require.NoError(t, err)
	require.Equal(t, "a", val, "copied doesn't retain original values")

	err = parent.Set("a", "b")
	require.NoError(t, err)
	val, err = env.Get("a")
	require.NoError(t, err)
	require.Equal(t, "b", val, "son was not modified")
	val, err = copied.Get("a")
	require.NoError(t, err)
	require.Equal(t, "a", val, "copied got the new value")

	err = parent.Set("a", "c")
	require.NoError(t, err)
	val, err = env.Get("a")
	require.NoError(t, err)
	require.Equal(t, "c", val, "original was not modified")
	val, err = copied.Get("a")
	require.NoError(t, err)
	require.Equal(t, "a", val, "copied was modified")

	err = parent.Define("b", "b")
	require.NoError(t, err)
	_, err = copied.Get("b")
	require.Error(t, err, "copied parent was modified")
}

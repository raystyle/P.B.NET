package env

import (
	"fmt"
	"reflect"
	"strings"
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

type mockExternalLookup struct {
	values map[string]reflect.Value
	types  map[string]reflect.Type
}

func testNewMockExternalLookup() *mockExternalLookup {
	return &mockExternalLookup{
		values: make(map[string]reflect.Value),
		types:  make(map[string]reflect.Type),
	}
}

func (mel *mockExternalLookup) SetValue(symbol string, value interface{}) error {
	if strings.Contains(symbol, ".") {
		return ErrSymbolContainsDot
	}
	if value == nil {
		mel.values[symbol] = NilValue
	} else {
		mel.values[symbol] = reflect.ValueOf(value)
	}
	return nil
}

func (mel *mockExternalLookup) Get(symbol string) (reflect.Value, error) {
	if value, ok := mel.values[symbol]; ok {
		return value, nil
	}
	return NilValue, fmt.Errorf("undefined symbol \"%s\"", symbol)
}

func (mel *mockExternalLookup) DefineType(symbol string, aType interface{}) error {
	if strings.Contains(symbol, ".") {
		return ErrSymbolContainsDot
	}
	var reflectType reflect.Type
	if aType == nil {
		reflectType = NilType
	} else {
		var ok bool
		reflectType, ok = reflectType.(reflect.Type)
		if !ok {
			reflectType = reflect.TypeOf(aType)
		}
	}
	mel.types[symbol] = reflectType
	return nil
}

func (mel *mockExternalLookup) Type(symbol string) (reflect.Type, error) {
	if value, ok := mel.types[symbol]; ok {
		return value, nil
	}
	return NilType, fmt.Errorf("undefined symbol \"%s\"", symbol)
}

func TestExternalLookupValueAndGet(t *testing.T) {
	tests := []struct {
		info        string
		name        string
		defineValue interface{}
		getValue    interface{}
		kind        reflect.Kind
		defineErr   error
		getErr      error
	}{
		{info: "nil", name: "a", defineValue: nil, getValue: nil, kind: reflect.Interface},
		{info: "bool", name: "a", defineValue: true, getValue: true, kind: reflect.Bool},
		{info: "int16", name: "a", defineValue: int16(1), getValue: int16(1), kind: reflect.Int16},
		{info: "int32", name: "a", defineValue: int32(1), getValue: int32(1), kind: reflect.Int32},
		{info: "int64", name: "a", defineValue: int64(1), getValue: int64(1), kind: reflect.Int64},
		{info: "uint32", name: "a", defineValue: uint32(1), getValue: uint32(1), kind: reflect.Uint32},
		{info: "uint64", name: "a", defineValue: uint64(1), getValue: uint64(1), kind: reflect.Uint64},
		{info: "float32", name: "a", defineValue: float32(1), getValue: float32(1), kind: reflect.Float32},
		{info: "float64", name: "a", defineValue: float64(1), getValue: float64(1), kind: reflect.Float64},
		{info: "string", name: "a", defineValue: "a", getValue: "a", kind: reflect.String},
		{
			info:        "string with dot",
			name:        "a.a",
			defineValue: "a",
			getValue:    nil,
			kind:        reflect.String,
			defineErr:   ErrSymbolContainsDot,
			getErr:      fmt.Errorf("undefined symbol \"a.a\""),
		},
		{
			info:        "string with quotes",
			name:        "a",
			defineValue: `"a"`,
			getValue:    `"a"`,
			kind:        reflect.String,
		},
	}

	for _, test := range tests {
		testExternalLookup := testNewMockExternalLookup()
		env := NewEnv()
		env.SetExternalLookup(testExternalLookup)

		err := testExternalLookup.SetValue(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - SetValue error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - SetValue error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		value, err := env.Get(test.name)
		if err != nil && test.getErr != nil {
			if err.Error() != test.getErr.Error() {
				const format = "%v - Get error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.getErr)
				continue
			}
		} else if err != test.getErr {
			const format = "%v - Get error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.getErr)
			continue
		}
		if value != test.getValue {
			const format = "%v - value check - received %#v expected: %#v"
			t.Errorf(format, test.info, value, test.getValue)
		}
	}
}

func TestExternalLookupTypeAndGet(t *testing.T) {
	tests := []struct {
		info        string
		name        string
		defineValue interface{}
		defineErr   error
		typeErr     error
	}{
		{info: "nil", name: "a", defineValue: nil},
		{info: "bool", name: "a", defineValue: true},
		{info: "int16", name: "a", defineValue: int16(1)},
		{info: "int32", name: "a", defineValue: int32(1)},
		{info: "int64", name: "a", defineValue: int64(1)},
		{info: "uint32", name: "a", defineValue: uint32(1)},
		{info: "uint64", name: "a", defineValue: uint64(1)},
		{info: "float32", name: "a", defineValue: float32(1)},
		{info: "float64", name: "a", defineValue: float64(1)},
		{info: "string", name: "a", defineValue: "a"},
		{
			info:        "string with dot",
			name:        "a.a",
			defineValue: nil,
			defineErr:   ErrSymbolContainsDot,
			typeErr:     fmt.Errorf("undefined type \"a.a\""),
		},
	}

	for _, test := range tests {
		testExternalLookup := testNewMockExternalLookup()
		env := NewEnv()
		env.SetExternalLookup(testExternalLookup)

		err := testExternalLookup.DefineType(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - DefineType error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - DefineType error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		valueType, err := env.Type(test.name)
		if err != nil && test.typeErr != nil {
			if err.Error() != test.typeErr.Error() {
				const format = "%v - Type error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.typeErr)
				continue
			}
		} else if err != test.typeErr {
			const format = "%v - Type error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.typeErr)
			continue
		}
		if valueType == nil || test.defineValue == nil {
			if valueType != reflect.TypeOf(test.defineValue) {
				const format = "%v - Type check - received: %v - expected: %v"
				t.Errorf(format, test.info, valueType, reflect.TypeOf(test.defineValue))
			}
		} else if valueType.String() != reflect.TypeOf(test.defineValue).String() {
			const format = "%v - Type check - received: %v - expected: %v"
			t.Errorf(format, test.info, valueType, reflect.TypeOf(test.defineValue))
		}
	}
}

func TestExternalLookupAddr(t *testing.T) {
	tests := []struct {
		info        string
		name        string
		defineValue interface{}
		defineErr   error
		addrErr     error
	}{
		{info: "nil", name: "a", defineValue: nil, addrErr: nil},
		{info: "bool", name: "a", defineValue: true, addrErr: fmt.Errorf("unaddressable")},
		{info: "int64", name: "a", defineValue: int64(1), addrErr: fmt.Errorf("unaddressable")},
		{info: "float64", name: "a", defineValue: float64(1), addrErr: fmt.Errorf("unaddressable")},
		{info: "string", name: "a", defineValue: "a", addrErr: fmt.Errorf("unaddressable")},
	}

	for _, test := range tests {
		envParent := NewEnv()
		testExternalLookup := testNewMockExternalLookup()
		envParent.SetExternalLookup(testExternalLookup)
		envChild := envParent.NewEnv()

		err := testExternalLookup.SetValue(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - SetValue error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - SetValue error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		_, err = envChild.Addr(test.name)
		if err != nil && test.addrErr != nil {
			if err.Error() != test.addrErr.Error() {
				const format = "%v - Addr error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.addrErr)
				continue
			}
		} else if err != test.addrErr {
			const format = "%v - Addr error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.addrErr)
			continue
		}
	}
}

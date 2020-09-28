package env

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicType(t *testing.T) {
	env := NewEnv()
	aType, err := env.Type("string")
	if err != nil {
		t.Fatalf("Type error - %v", err)
	}
	if aType != reflect.TypeOf("a") {
		t.Errorf("Type - received: %v - expected: %v", aType, reflect.TypeOf("a"))
	}

	aType, err = env.Type("int64")
	if err != nil {
		t.Fatal("Type error:", err)
	}
	if aType != reflect.TypeOf(int64(1)) {
		t.Errorf("Type - received: %v - expected: %v", aType, reflect.TypeOf(int64(1)))
	}
}

func TestEnv_DefineType(t *testing.T) {
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

	env := NewEnv()
	err := env.DefineGlobalType("a", "a")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		env := NewEnv()

		err := env.DefineType(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - Define error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - Define error - received: %v - expected: %v"
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

func TestEnv_DefineType_NewEnv(t *testing.T) {
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
		envParent := NewEnv()
		envChild := envParent.NewEnv()

		err := envParent.DefineType(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - Define error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - Define error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		valueType, err := envChild.Type(test.name)
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

func TestEnv_DefineType_NewModule(t *testing.T) {
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
		envParent := NewEnv()
		envChild, err := envParent.NewModule("envChild")
		if err != nil {
			const format = "%v - NewModule error - received: %v - expected: %v"
			t.Fatalf(format, test.info, err, nil)
		}

		err = envParent.DefineType(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - Define error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - Define error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		valueType, err := envChild.Type(test.name)
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

func TestEnv_DefineGlobalType_Parent(t *testing.T) {
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
		envParent := NewEnv()
		envChild := envParent.NewEnv()

		err := envChild.DefineGlobalType(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - Define error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - Define error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		valueType, err := envParent.Type(test.name)
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

func TestEnv_DefineGlobalType_Child(t *testing.T) {
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
		envParent := NewEnv()
		envChild := envParent.NewEnv()

		err := envChild.DefineGlobalType(test.name, test.defineValue)
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - Define error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - Define error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		valueType, err := envChild.Type(test.name)
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

func TestEnv_DefineGlobalReflectType_Parent(t *testing.T) {
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

	env := NewEnv()
	err := env.DefineGlobalReflectType("a", reflect.TypeOf("a"))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		envParent := NewEnv()
		envChild := envParent.NewEnv()

		err := envChild.DefineGlobalReflectType(test.name, reflect.TypeOf(test.defineValue))
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - Define error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - Define error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		valueType, err := envParent.Type(test.name)
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

func TestEnv_DefineGlobalReflectType_Child(t *testing.T) {
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
		envParent := NewEnv()
		envChild := envParent.NewEnv()

		err := envChild.DefineGlobalReflectType(test.name, reflect.TypeOf(test.defineValue))
		if err != nil && test.defineErr != nil {
			if err.Error() != test.defineErr.Error() {
				const format = "%v - Define error - received: %v - expected: %v"
				t.Errorf(format, test.info, err, test.defineErr)
				continue
			}
		} else if err != test.defineErr {
			const format = "%v - Define error - received: %v - expected: %v"
			t.Errorf(format, test.info, err, test.defineErr)
			continue
		}

		valueType, err := envChild.Type(test.name)
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

func TestEnv_DeleteType(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		env := NewEnv()
		env.DeleteType("a")
	})

	t.Run("add & delete", func(t *testing.T) {
		env := NewEnv()
		err := env.DefineType("a", "a")
		require.NoError(t, err)
		env.DeleteType("a")

		typ, err := env.Type("a")
		require.EqualError(t, err, "undefined type \"a\"")
		require.Nil(t, typ)
	})
}

func TestEnv_DeleteGlobalType(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		env := NewEnv()
		env.DeleteGlobal("a")
	})

	t.Run("add & delete", func(t *testing.T) {
		env := NewEnv()
		err := env.DefineType("a", "a")
		require.NoError(t, err)
		env.DeleteGlobalType("a")

		typ, err := env.Type("a")
		require.EqualError(t, err, "undefined type \"a\"")
		require.Nil(t, typ)
	})

	t.Run("parent & child, var in child, delete in parent", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := envChild.DefineType("a", "a")
		require.NoError(t, err)
		env.DeleteGlobalType("a")

		typ, err := envChild.Type("a")
		require.NoError(t, err)
		require.Equal(t, typ.String(), "string")

		envChild.DeleteGlobalType("a")
		typ, err = env.Type("a")
		require.EqualError(t, err, "undefined type \"a\"")
		require.Nil(t, typ)
	})

	t.Run("parent & child, var in child, delete in child", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := envChild.DefineType("a", "a")
		require.NoError(t, err)
		env.DeleteGlobalType("a")

		envChild.DeleteGlobalType("a")
		typ, err := envChild.Type("a")
		require.EqualError(t, err, "undefined type \"a\"")
		require.Nil(t, typ)
	})

	t.Run("parent & child, var in parent, delete in child", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := env.DefineType("a", "a")
		require.NoError(t, err)

		envChild.DeleteGlobalType("a")

		typ, err := envChild.Type("a")
		require.EqualError(t, err, "undefined type \"a\"")
		require.Nil(t, typ)
	})

	t.Run("parent & child, var in parent, delete in parent", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := env.DefineType("a", "a")
		require.NoError(t, err)
		env.DeleteGlobalType("a")

		typ, err := envChild.Type("a")
		require.EqualError(t, err, "undefined type \"a\"")
		require.Nil(t, typ)
	})
}

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

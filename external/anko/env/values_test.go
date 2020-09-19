package env

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnv_DefineGlobal(t *testing.T) {
	envParent := NewEnv()
	envChild := envParent.NewEnv()
	err := envChild.DefineGlobal("a", "a")
	if err != nil {
		t.Fatal("DefineGlobal error:", err)
	}

	var value interface{}
	value, err = envParent.Get("a")
	if err != nil {
		t.Fatal("Get error:", err)
	}
	v, ok := value.(string)
	if !ok {
		t.Fatalf("value - received: %T - expected: %T", value, "a")
	}
	if v != "a" {
		t.Fatalf("value - received: %v - expected: %v", v, "a")
	}
}

func TestEnv_DefineGlobalValue(t *testing.T) {
	envParent := NewEnv()
	envChild := envParent.NewEnv()
	err := envChild.DefineGlobalValue("a", reflect.ValueOf("a"))
	if err != nil {
		t.Fatal("DefineGlobalValue error:", err)
	}

	var value interface{}
	value, err = envParent.Get("a")
	if err != nil {
		t.Fatal("Get error:", err)
	}
	v, ok := value.(string)
	if !ok {
		t.Fatalf("value - received: %T - expected: %T", value, "a")
	}
	if v != "a" {
		t.Fatalf("value - received: %v - expected: %v", v, "a")
	}
}

func TestEnv_Addr(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		tests := []struct {
			info        string
			name        string
			defineValue interface{}
			defineErr   error
			addrErr     error
		}{
			{info: "nil", name: "a", defineValue: nil, addrErr: nil},
			{info: "string", name: "a", defineValue: "a", addrErr: fmt.Errorf("unaddressable")},
			{info: "int64", name: "a", defineValue: int64(1), addrErr: fmt.Errorf("unaddressable")},
			{info: "float64", name: "a", defineValue: float64(1), addrErr: fmt.Errorf("unaddressable")},
			{info: "bool", name: "a", defineValue: true, addrErr: fmt.Errorf("unaddressable")},
		}

		for _, test := range tests {
			envParent := NewEnv()
			envChild := envParent.NewEnv()

			err := envParent.Define(test.name, test.defineValue)
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
	})

	t.Run("error", func(t *testing.T) {
		envParent := NewEnv()
		envChild := envParent.NewEnv()
		_, err := envChild.Addr("a")
		require.EqualError(t, err, "undefined symbol \"a\"")
	})
}

func TestEnv_Values(t *testing.T) {
	env := NewEnv()
	err := env.Define("test", "test str")
	require.NoError(t, err)

	values := env.Values()
	v, ok := values["test"]
	require.True(t, ok)
	require.Equal(t, "test str", v.Interface().(string))
}

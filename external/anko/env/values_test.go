package env

import (
	"fmt"
	"reflect"
	"sync"
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

func TestEnv_DefineAndGet(t *testing.T) {
	tests := []struct {
		info        string
		name        string
		defineValue interface{}
		getValue    interface{}
		kind        reflect.Kind
		defineErr   error
		getErr      error
	}{
		{info: "nil", name: "a", defineValue: reflect.Value{}, getValue: reflect.Value{}, kind: reflect.Invalid},
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
			kind:        reflect.Interface,
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
		env := NewEnv()

		err := env.Define(test.name, test.defineValue)
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

func TestEnv_DefineAndGet_NewEnv(t *testing.T) {
	tests := []struct {
		info        string
		name        string
		defineValue interface{}
		getValue    interface{}
		kind        reflect.Kind
		defineErr   error
		getErr      error
	}{
		{info: "nil", name: "a", defineValue: reflect.Value{}, getValue: reflect.Value{}, kind: reflect.Invalid},
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
			kind:        reflect.Interface,
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

		value, err := envChild.Get(test.name)
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

func TestEnv_DefineAndGet_DefineGlobal(t *testing.T) {
	tests := []struct {
		info        string
		name        string
		defineValue interface{}
		getValue    interface{}
		kind        reflect.Kind
		defineErr   error
		getErr      error
	}{
		{info: "nil", name: "a", defineValue: reflect.Value{}, getValue: reflect.Value{}, kind: reflect.Invalid},
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
			kind:        reflect.Interface,
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
		envParent := NewEnv()
		envChild := envParent.NewEnv()

		err := envChild.DefineGlobal(test.name, test.defineValue)
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

		value, err := envParent.Get(test.name)
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

func TestEnv_Define_Modify(t *testing.T) {
	tests := []struct {
		info        string
		name        string
		defineValue interface{}
		getValue    interface{}
		kind        reflect.Kind
	}{
		{info: "nil", name: "a", defineValue: nil, getValue: nil, kind: reflect.Interface},
		{info: "bool", name: "a", defineValue: true, getValue: true, kind: reflect.Bool},
		{info: "int64", name: "a", defineValue: int64(1), getValue: int64(1), kind: reflect.Int64},
		{info: "float64", name: "a", defineValue: float64(1), getValue: float64(1), kind: reflect.Float64},
		{info: "string", name: "a", defineValue: "a", getValue: "a", kind: reflect.String},
	}
	changeTests := []struct {
		defineValue interface{}
		getValue    interface{}
		kind        reflect.Kind
	}{
		{defineValue: nil, getValue: nil, kind: reflect.Interface},
		{defineValue: "a", getValue: "a", kind: reflect.String},
		{defineValue: int64(1), getValue: int64(1), kind: reflect.Int64},
		{defineValue: float64(1), getValue: float64(1), kind: reflect.Float64},
		{defineValue: true, getValue: true, kind: reflect.Bool},
	}

	t.Run("common", func(t *testing.T) {
		for _, test := range tests {
			env := NewEnv()

			err := env.Define(test.name, test.defineValue)
			require.NoError(t, err)
			value, err := env.Get(test.name)
			require.NoError(t, err)
			require.Equal(t, test.getValue, value)

			for _, changeTest := range changeTests {
				err = env.Set(test.name, changeTest.defineValue)
				require.NoError(t, err)
				value, err = env.Get(test.name)
				require.NoError(t, err)
				require.Equal(t, changeTest.getValue, value)
			}
		}
	})

	t.Run("envParent", func(t *testing.T) {
		for _, test := range tests {
			envParent := NewEnv()
			envChild := envParent.NewEnv()

			err := envParent.Define(test.name, test.defineValue)
			require.NoError(t, err)
			value, err := envChild.Get(test.name)
			require.NoError(t, err)
			require.Equal(t, test.getValue, value)

			for _, changeTest := range changeTests {
				err = envParent.Set(test.name, changeTest.defineValue)
				require.NoError(t, err)
				value, err = envChild.Get(test.name)
				require.NoError(t, err)
				require.Equal(t, changeTest.getValue, value)
			}
		}
	})

	t.Run("envChild", func(t *testing.T) {
		for _, test := range tests {
			envParent := NewEnv()
			envChild := envParent.NewEnv()

			err := envParent.Define(test.name, test.defineValue)
			require.NoError(t, err)
			value, err := envChild.Get(test.name)
			require.NoError(t, err)
			require.Equal(t, test.getValue, value)

			for _, changeTest := range changeTests {
				err = envChild.Set(test.name, changeTest.defineValue)
				require.NoError(t, err)
				value, err = envChild.Get(test.name)
				require.NoError(t, err)
				require.Equal(t, changeTest.getValue, value)
			}
		}
	})
}

func TestRaceCreateSameVariable(t *testing.T) {
	// Test creating same variable in parallel

	waitChan := make(chan struct{}, 1)
	var waitGroup sync.WaitGroup

	env := NewEnv()

	for i := 0; i < 100; i++ {
		waitGroup.Add(1)
		go func(i int) {
			<-waitChan
			err := env.Define("a", i)
			if err != nil {
				t.Errorf("Define error: %v", err)
			}
			_, err = env.Get("a")
			if err != nil {
				t.Errorf("Get error: %v", err)
			}
			waitGroup.Done()
		}(i)
	}

	close(waitChan)
	waitGroup.Wait()

	_, err := env.Get("a")
	if err != nil {
		t.Errorf("Get error: %v", err)
	}
}

func TestRaceCreateDifferentVariables(t *testing.T) {
	// Test creating different variables in parallel

	waitChan := make(chan struct{}, 1)
	var waitGroup sync.WaitGroup

	env := NewEnv()

	for i := 0; i < 100; i++ {
		waitGroup.Add(1)
		go func(i int) {
			<-waitChan
			err := env.Define(fmt.Sprint(i), i)
			if err != nil {
				t.Errorf("Define error: %v", err)
			}
			_, err = env.Get(fmt.Sprint(i))
			if err != nil {
				t.Errorf("Get error: %v", err)
			}
			waitGroup.Done()
		}(i)
	}

	close(waitChan)
	waitGroup.Wait()

	for i := 0; i < 100; i++ {
		_, err := env.Get(fmt.Sprint(i))
		if err != nil {
			t.Errorf("Get error: %v", err)
		}
	}
}

func TestRaceReadDifferentVariables(t *testing.T) {
	// Test reading different variables in parallel

	waitChan := make(chan struct{}, 1)
	var waitGroup sync.WaitGroup

	env := NewEnv()

	for i := 0; i < 100; i++ {
		err := env.Define(fmt.Sprint(i), i)
		if err != nil {
			t.Errorf("Define error: %v", err)
		}
		_, err = env.Get(fmt.Sprint(i))
		if err != nil {
			t.Errorf("Get error: %v", err)
		}
	}

	for i := 0; i < 100; i++ {
		waitGroup.Add(1)
		go func(i int) {
			<-waitChan
			_, err := env.Get(fmt.Sprint(i))
			if err != nil {
				t.Errorf("Get error: %v", err)
			}
			waitGroup.Done()
		}(i)
	}

	close(waitChan)
	waitGroup.Wait()
}

func TestRaceSetSameVariable(t *testing.T) {
	// Test setting same variable in parallel

	waitChan := make(chan struct{}, 1)
	var waitGroup sync.WaitGroup

	env := NewEnv()

	err := env.Define("a", 0)
	if err != nil {
		t.Errorf("Define error: %v", err)
	}
	_, err = env.Get("a")
	if err != nil {
		t.Errorf("Get error: %v", err)
	}

	for i := 0; i < 100; i++ {
		waitGroup.Add(1)
		go func(i int) {
			<-waitChan
			err := env.Set("a", i)
			if err != nil {
				t.Errorf("Set error: %v", err)
			}
			waitGroup.Done()
		}(i)
	}

	close(waitChan)
	waitGroup.Wait()

	_, err = env.Get("a")
	if err != nil {
		t.Errorf("Get error: %v", err)
	}
}

func TestRaceSetSameVariableNewEnv(t *testing.T) {
	// Test setting same variable in parallel with NewEnv

	waitChan := make(chan struct{}, 1)
	var waitGroup sync.WaitGroup

	env := NewEnv()

	err := env.Define("a", 0)
	if err != nil {
		t.Errorf("Define error: %v", err)
	}
	_, err = env.Get("a")
	if err != nil {
		t.Errorf("Get error: %v", err)
	}

	for i := 0; i < 100; i++ {
		waitGroup.Add(1)
		go func(i int) {
			<-waitChan
			env = env.NewEnv().NewEnv()
			err := env.Set("a", i)
			if err != nil {
				t.Errorf("Set error: %v", err)
			}
			waitGroup.Done()
		}(i)
	}
}

func TestRaceDefineAndSetSameVariable(t *testing.T) {
	// Test defining and setting same variable in parallel
	for i := 0; i < 100; i++ {
		raceDefineAndSetSameVariable(t)
	}
}

func raceDefineAndSetSameVariable(t *testing.T) {
	waitChan := make(chan struct{}, 1)
	var waitGroup sync.WaitGroup

	envParent := NewEnv()
	envChild := envParent.NewEnv()

	for i := 0; i < 2; i++ {
		waitGroup.Add(1)
		go func() {
			<-waitChan
			err := envParent.Set("a", 1)
			if err != nil && err.Error() != "undefined symbol \"a\"" {
				t.Errorf("Set error: %v", err)
			}
			waitGroup.Done()
		}()
		waitGroup.Add(1)
		go func() {
			<-waitChan
			err := envParent.Define("a", 2)
			if err != nil {
				t.Errorf("Define error: %v", err)
			}
			waitGroup.Done()
		}()
		waitGroup.Add(1)
		go func() {
			<-waitChan
			err := envChild.Set("a", 3)
			if err != nil && err.Error() != "undefined symbol \"a\"" {
				t.Errorf("Set error: %v", err)
			}
			waitGroup.Done()
		}()
		waitGroup.Add(1)
		go func() {
			<-waitChan
			err := envChild.Define("a", 4)
			if err != nil {
				t.Errorf("Define error: %v", err)
			}
			waitGroup.Done()
		}()
	}

	close(waitChan)
	waitGroup.Wait()

	_, err := envParent.Get("a") // value of a could be 1, 2, or 3
	if err != nil {
		t.Errorf("Get error: %v", err)
	}
	_, err = envChild.Get("a") // value of a could be 3 or 4
	if err != nil {
		t.Errorf("Get error: %v", err)
	}
}

func TestEnv_Delete(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		env := NewEnv()
		env.Delete("a")
	})

	t.Run("add & delete", func(t *testing.T) {
		env := NewEnv()
		err := env.Define("a", "a")
		require.NoError(t, err)
		env.Delete("a")

		value, err := env.Get("a")
		require.EqualError(t, err, "undefined symbol \"a\"")
		require.Nil(t, value)
	})
}

func TestEnv_DeleteGlobal(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		env := NewEnv()
		env.DeleteGlobal("a")
	})

	t.Run("add & delete", func(t *testing.T) {
		env := NewEnv()
		err := env.Define("a", "a")
		require.NoError(t, err)
		env.DeleteGlobal("a")

		value, err := env.Get("a")
		require.EqualError(t, err, "undefined symbol \"a\"")
		require.Nil(t, value)
	})

	t.Run("parent & child, var in child, delete in parent", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := envChild.Define("a", "a")
		require.NoError(t, err)
		env.DeleteGlobal("a")

		value, err := envChild.Get("a")
		require.NoError(t, err)
		require.Equal(t, value, "a")

		envChild.DeleteGlobal("a")
		value, err = env.Get("a")
		require.EqualError(t, err, "undefined symbol \"a\"")
		require.Nil(t, value)
	})

	t.Run("parent & child, var in child, delete in child", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := envChild.Define("a", "a")
		require.NoError(t, err)
		env.DeleteGlobal("a")

		envChild.DeleteGlobal("a")

		value, err := envChild.Get("a")
		require.EqualError(t, err, "undefined symbol \"a\"")
		require.Nil(t, value)
	})

	t.Run("parent & child, var in parent, delete in child", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := env.Define("a", "a")
		require.NoError(t, err)

		envChild.DeleteGlobal("a")

		value, err := envChild.Get("a")
		require.EqualError(t, err, "undefined symbol \"a\"")
		require.Nil(t, value)
	})

	t.Run("parent & child, var in parent, delete in parent", func(t *testing.T) {
		env := NewEnv()
		envChild := env.NewEnv()
		err := env.Define("a", "a")
		require.NoError(t, err)
		env.DeleteGlobal("a")

		value, err := envChild.Get("a")
		require.EqualError(t, err, "undefined symbol \"a\"")
		require.Nil(t, value)
	})
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

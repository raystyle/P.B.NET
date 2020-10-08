package core

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"project/external/anko/env"
	"project/external/anko/parser"
	"project/external/anko/vm"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

// Test is utility struct to make tests easy.
type Test struct {
	Script     string
	ParseError error
	Types      map[string]interface{}
	Input      map[string]interface{}
	RunError   error
	RunOutput  interface{}
	Output     map[string]interface{}
}

// TestOptions is utility struct to pass options to the test.
type TestOptions struct {
	EnvSetupFunc func(*env.Env)
	Timeout      time.Duration
}

func runTests(t *testing.T, tests []*Test, testOpts *TestOptions, opts *vm.Options) {
	for _, test := range tests {
		runTest(t, test, testOpts, opts)
	}
}

// nolint: gocyclo
//gocyclo:ignore
func runTest(t *testing.T, test *Test, testOpts *TestOptions, opts *vm.Options) {
	timeout := 60 * time.Second

	// parser.EnableErrorVerbose()
	// parser.EnableDebug(8)

	stmt, err := parser.ParseSrc(test.Script)
	if err != nil && test.ParseError != nil {
		if err.Error() != test.ParseError.Error() {
			const format = "ParseSrc error - received: %v - expected: %v - script: %v"
			t.Errorf(format, err, test.ParseError, test.Script)
			return
		}
	} else if err != test.ParseError {
		const format = "ParseSrc error - received: %v - expected: %v - script: %v"
		t.Errorf(format, err, test.ParseError, test.Script)
		return
	}
	// Note: Still want to run the code even after a parse error to see what happens

	envTest := env.NewEnv()
	if testOpts != nil {
		if testOpts.EnvSetupFunc != nil {
			testOpts.EnvSetupFunc(envTest)
		}
		if testOpts.Timeout != 0 {
			timeout = testOpts.Timeout
		}
	}

	for typeName, typeValue := range test.Types {
		err = envTest.DefineType(typeName, typeValue)
		if err != nil {
			t.Errorf("DefineType error: %v - typeName: %v - script: %v", err, typeName, test.Script)
			return
		}
	}

	for inputName, inputValue := range test.Input {
		err = envTest.Define(inputName, inputValue)
		if err != nil {
			t.Errorf("Define error: %v - inputName: %v - script: %v", err, inputName, test.Script)
			return
		}
	}

	var value interface{}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	value, err = vm.RunContext(ctx, envTest, opts, stmt)
	cancel()
	if err != nil && test.RunError != nil {
		if err.Error() != test.RunError.Error() {
			t.Errorf("Run error - received: %v - expected: %v - script: %v", err, test.RunError, test.Script)
			return
		}
	} else if err != test.RunError {
		t.Errorf("Run error - received: %v - expected: %v - script: %v", err, test.RunError, test.Script)
		return
	}

	if !valueEqual(value, test.RunOutput) {
		t.Errorf("Run output - received: %#v - expected: %#v - script: %v", value, test.RunOutput, test.Script)
		t.Errorf("received type: %T - expected: %T", value, test.RunOutput)
		return
	}

	for outputName, outputValue := range test.Output {
		value, err = envTest.Get(outputName)
		if err != nil {
			t.Errorf("Get error: %v - outputName: %v - script: %v", err, outputName, test.Script)
			return
		}

		if !valueEqual(value, outputValue) {
			const format = "outputName %v - received: %#v - expected: %#v - script: %v"
			t.Errorf(format, outputName, value, outputValue, test.Script)
			t.Errorf("received type: %T - expected: %T", value, outputValue)
			continue
		}
	}
}

// valueEqual return true if v1 and v2 is same value. If passed function, does
// extra checks otherwise just doing reflect.DeepEqual.
func valueEqual(v1 interface{}, v2 interface{}) bool {
	v1RV := reflect.ValueOf(v1)
	switch v1RV.Kind() {
	case reflect.Func:
		// This is best effort to check if functions match, but it could be wrong
		v2RV := reflect.ValueOf(v2)
		if !v1RV.IsValid() || !v2RV.IsValid() {
			if v1RV.IsValid() != !v2RV.IsValid() {
				return false
			}
			return true
		} else if v1RV.Kind() != v2RV.Kind() {
			return false
		} else if v1RV.Type() != v2RV.Type() {
			return false
		} else if v1RV.Pointer() != v2RV.Pointer() {
			// From reflect: If v's Kind is Func, the returned pointer is an underlying
			// code pointer, but not necessarily enough to identify a single function uniquely.
			return false
		}
		return true
	}
	switch value1 := v1.(type) {
	case error:
		switch value2 := v2.(type) {
		case error:
			return value1.Error() == value2.Error()
		}
	}

	return reflect.DeepEqual(v1, v2)
}

func TestImport(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		e := env.NewEnv()
		Import(e)
	})

	t.Run("failed", func(t *testing.T) {
		e := env.NewEnv()
		patch := func(interface{}, string, interface{}) error {
			return monkey.Error
		}
		pg := monkey.PatchInstanceMethod(e, "Define", patch)
		defer pg.Unpatch()

		defer testsuite.DeferForPanic(t)
		Import(e)
	})
}

func TestCoreKeys(t *testing.T) {
	tests := []*Test{
		{Script: `a = {}; b = keys(a)`, RunOutput: []interface{}{}, Output: map[string]interface{}{"a": map[interface{}]interface{}{}}},
		{Script: `a = {"a": nil}; b = keys(a)`, RunOutput: []interface{}{"a"}, Output: map[string]interface{}{"a": map[interface{}]interface{}{"a": nil}}},
		{Script: `a = {"a": 1}; b = keys(a)`, RunOutput: []interface{}{"a"}, Output: map[string]interface{}{"a": map[interface{}]interface{}{"a": int64(1)}}},
	}
	runTests(t, tests, &TestOptions{EnvSetupFunc: Import}, &vm.Options{Debug: true})
}

func TestCoreRange(t *testing.T) {
	tests := []*Test{
		// 0 arguments
		{Script: `range()`, RunError: fmt.Errorf("range expected at least 1 argument, got 0")},
		// 1 arguments(step == 1, start == 0)
		{Script: `range(-1)`, RunOutput: []int64(nil)},
		{Script: `range(0)`, RunOutput: []int64(nil)},
		{Script: `range(1)`, RunOutput: []int64{0}},
		{Script: `range(2)`, RunOutput: []int64{0, 1}},
		{Script: `range(10)`, RunOutput: []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		// 2 arguments(step == 1)
		{Script: `range(-5,-1)`, RunOutput: []int64{-5, -4, -3, -2}},
		{Script: `range(-1,1)`, RunOutput: []int64{-1, 0}},
		{Script: `range(1,5)`, RunOutput: []int64{1, 2, 3, 4}},
		// 3 arguments
		// step == 2
		{Script: `range(-5,-1,2)`, RunOutput: []int64{-5, -3}},
		{Script: `range(1,5,2)`, RunOutput: []int64{1, 3}},
		{Script: `range(-1,5,2)`, RunOutput: []int64{-1, 1, 3}},
		// step < 0 and from small to large
		{Script: `range(-5,-1,-1)`, RunOutput: []int64(nil)},
		{Script: `range(1,5,-1)`, RunOutput: []int64(nil)},
		{Script: `range(-1,5,-1)`, RunOutput: []int64(nil)},
		// step < 0 and from large to small
		{Script: `range(-1,-5,-1)`, RunOutput: []int64{-1, -2, -3, -4}},
		{Script: `range(5,1,-1)`, RunOutput: []int64{5, 4, 3, 2}},
		{Script: `range(5,-1,-1)`, RunOutput: []int64{5, 4, 3, 2, 1, 0}},
		// 4,5 arguments
		{Script: `range(1,5,1,1)`, RunError: fmt.Errorf("range expected at most 3 arguments, got 4")},
		{Script: `range(1,5,1,1,1)`, RunError: fmt.Errorf("range expected at most 3 arguments, got 5")},
		// more 0 test
		{Script: `range(0,1,2)`, RunOutput: []int64{0}},
		{Script: `range(1,0,2)`, RunOutput: []int64(nil)},
		{Script: `range(1,2,0)`, RunError: fmt.Errorf("range argument 3 must not be zero")},
	}
	runTests(t, tests, &TestOptions{EnvSetupFunc: Import}, &vm.Options{Debug: false})
}

func TestCoreTypeOf(t *testing.T) {

}

func TestCoreKindOf(t *testing.T) {

}

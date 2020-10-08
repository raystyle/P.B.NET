package core

import (
	"context"
	"reflect"
	"testing"
	"time"

	"project/external/anko/env"
	"project/external/anko/parser"
	"project/external/anko/vm"
)

// Test is utility struct to make tests easy.
type Test struct {
	Script         string
	ParseError     error
	ParseErrorFunc *func(*testing.T, error)
	EnvSetupFunc   *func(*testing.T, *env.Env)
	Types          map[string]interface{}
	Input          map[string]interface{}
	RunError       error
	RunErrorFunc   *func(*testing.T, error)
	RunOutput      interface{}
	Output         map[string]interface{}
}

// TestOptions is utility struct to pass options to the test.
type TestOptions struct {
	EnvSetupFunc *func(*testing.T, *env.Env)
	Timeout      time.Duration
}

func runTests(t *testing.T, tests []Test, testOptions *TestOptions, options *vm.Options) {
	for _, test := range tests {
		runTest(t, test, testOptions, options)
	}
}

// nolint: gocyclo
//gocyclo:ignore
func runTest(t *testing.T, test Test, testOptions *TestOptions, options *vm.Options) {
	timeout := 60 * time.Second

	// parser.EnableErrorVerbose()
	// parser.EnableDebug(8)

	stmt, err := parser.ParseSrc(test.Script)
	if test.ParseErrorFunc != nil {
		(*test.ParseErrorFunc)(t, err)
	} else if err != nil && test.ParseError != nil {
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
	if testOptions != nil {
		if testOptions.EnvSetupFunc != nil {
			(*testOptions.EnvSetupFunc)(t, envTest)
		}
		if testOptions.Timeout != 0 {
			timeout = testOptions.Timeout
		}
	}
	if test.EnvSetupFunc != nil {
		(*test.EnvSetupFunc)(t, envTest)
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
	value, err = vm.RunContext(ctx, envTest, options, stmt)
	cancel()
	if test.RunErrorFunc != nil {
		(*test.RunErrorFunc)(t, err)
	} else if err != nil && test.RunError != nil {
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

package vm

import (
	"fmt"
	"reflect"
	"testing"

	"project/external/anko/env"
)

func TestImport(t *testing.T) {
	tests := []Test{
		{Script: `a = import(1++)`, RunError: fmt.Errorf("invalid operation")},
		{Script: `a = import(true)`, RunError: fmt.Errorf("invalid type conversion")},
		{Script: `a = import("foo")`, RunError: fmt.Errorf("package not found: foo")},
	}
	runTests(t, tests, nil, &Options{Debug: true})

	envPackages := env.Packages
	envPackageTypes := env.PackageTypes

	env.Packages = map[string]map[string]reflect.Value{"testPackage": {"a.b": reflect.ValueOf(1)}}
	tests = []Test{
		{Script: `a = import("testPackage")`, RunError: fmt.Errorf("import DefineValue error: symbol contains '.'")},
	}
	runTests(t, tests, nil, &Options{Debug: true})

	env.Packages = map[string]map[string]reflect.Value{"testPackage": {"a": reflect.ValueOf(1)}}
	env.PackageTypes = map[string]map[string]reflect.Type{"testPackage": {"a.b": reflect.TypeOf(1)}}
	tests = []Test{
		{Script: `a = import("testPackage")`, RunError: fmt.Errorf("import DefineReflectType error: symbol contains '.'")},
	}
	runTests(t, tests, nil, &Options{Debug: true})

	env.PackageTypes = envPackageTypes
	env.Packages = envPackages
}

func TestPackagesBytes(t *testing.T) {
	tests := []Test{
		{Script: `bytes = import("bytes"); a = make(bytes.Buffer); n, err = a.WriteString("a"); if err != nil { return err }; n`, RunOutput: 1},
		{Script: `bytes = import("bytes"); a = make(bytes.Buffer); n, err = a.WriteString("a"); if err != nil { return err }; a.String()`, RunOutput: "a"},
	}
	runTests(t, tests, nil, &Options{Debug: true})
}

func TestPackagesJson(t *testing.T) {
	tests := []Test{
		{Script: `json = import("encoding/json"); a = make(map[string]interface); a["b"] = "b"; c, err = json.Marshal(a); err`, Output: map[string]interface{}{"a": map[string]interface{}{"b": "b"}, "c": []byte(`{"b":"b"}`)}},
		{Script: `json = import("encoding/json"); b = 1; err = json.Unmarshal(a, &b); err`, Input: map[string]interface{}{"a": []byte(`{"b": "b"}`)}, Output: map[string]interface{}{"a": []byte(`{"b": "b"}`), "b": map[string]interface{}{"b": "b"}}},
		{Script: `json = import("encoding/json"); b = 1; err = json.Unmarshal(a, &b); err`, Input: map[string]interface{}{"a": `{"b": "b"}`}, Output: map[string]interface{}{"a": `{"b": "b"}`, "b": map[string]interface{}{"b": "b"}}},
		{Script: `json = import("encoding/json"); b = 1; err = json.Unmarshal(a, &b); err`, Input: map[string]interface{}{"a": `[["1", "2"],["3", "4"]]`}, Output: map[string]interface{}{"a": `[["1", "2"],["3", "4"]]`, "b": []interface{}{[]interface{}{"1", "2"}, []interface{}{"3", "4"}}}},
	}
	runTests(t, tests, nil, &Options{Debug: true})
}

func TestPackagesRegexp(t *testing.T) {
	tests := []Test{
		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^simple$"); re.MatchString("simple")`, RunOutput: true},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^simple$"); re.MatchString("no match")`, RunOutput: false},

		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.MatchString(b)`, Input: map[string]interface{}{"a": "^simple$", "b": "simple"}, RunOutput: true, Output: map[string]interface{}{"a": "^simple$", "b": "simple"}},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.MatchString(b)`, Input: map[string]interface{}{"a": "^simple$", "b": "no match"}, RunOutput: false, Output: map[string]interface{}{"a": "^simple$", "b": "no match"}},

		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^a\\.\\d+\\.b$"); re.String()`, RunOutput: "^a\\.\\d+\\.b$"},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^a\\.\\d+\\.b$"); re.MatchString("a.1.b")`, RunOutput: true},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^a\\.\\d+\\.b$"); re.MatchString("a.22.b")`, RunOutput: true},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^a\\.\\d+\\.b$"); re.MatchString("a.333.b")`, RunOutput: true},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^a\\.\\d+\\.b$"); re.MatchString("no match")`, RunOutput: false},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile("^a\\.\\d+\\.b$"); re.MatchString("a+1+b")`, RunOutput: false},

		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.String()`, Input: map[string]interface{}{"a": "^a\\.\\d+\\.b$"}, RunOutput: "^a\\.\\d+\\.b$", Output: map[string]interface{}{"a": "^a\\.\\d+\\.b$"}},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.MatchString(b)`, Input: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a.1.b"}, RunOutput: true, Output: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a.1.b"}},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.MatchString(b)`, Input: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a.22.b"}, RunOutput: true, Output: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a.22.b"}},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.MatchString(b)`, Input: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a.333.b"}, RunOutput: true, Output: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a.333.b"}},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.MatchString(b)`, Input: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "no match"}, RunOutput: false, Output: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "no match"}},
		{Script: `regexp = import("regexp"); re = regexp.MustCompile(a); re.MatchString(b)`, Input: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a+1+b"}, RunOutput: false, Output: map[string]interface{}{"a": "^a\\.\\d+\\.b$", "b": "a+1+b"}},
	}
	runTests(t, tests, nil, &Options{Debug: true})
}

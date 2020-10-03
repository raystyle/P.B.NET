package vm

import (
	"fmt"
	"reflect"
	"testing"
)

func TestReturns(t *testing.T) {
	tests := []Test{
		{Script: `return 1++`, RunError: fmt.Errorf("invalid operation")},
		{Script: `return 1, 1++`, RunError: fmt.Errorf("invalid operation")},
		{Script: `return 1, 2, 1++`, RunError: fmt.Errorf("invalid operation")},

		{Script: `return`, RunOutput: nil},
		{Script: `return nil`, RunOutput: nil},
		{Script: `return true`, RunOutput: true},
		{Script: `return 1`, RunOutput: int64(1)},
		{Script: `return 1.1`, RunOutput: 1.1},
		{Script: `return "a"`, RunOutput: "a"},

		{Script: `b()`, Input: map[string]interface{}{"b": func() {}}, RunOutput: nil},
		{Script: `b()`, Input: map[string]interface{}{"b": func() reflect.Value { return reflect.Value{} }}, RunOutput: reflect.Value{}},
		{Script: `b()`, Input: map[string]interface{}{"b": func() interface{} { return nil }}, RunOutput: nil},
		{Script: `b()`, Input: map[string]interface{}{"b": func() bool { return true }}, RunOutput: true},
		{Script: `b()`, Input: map[string]interface{}{"b": func() int32 { return int32(1) }}, RunOutput: int32(1)},
		{Script: `b()`, Input: map[string]interface{}{"b": func() int64 { return int64(1) }}, RunOutput: int64(1)},
		{Script: `b()`, Input: map[string]interface{}{"b": func() float32 { return float32(1.1) }}, RunOutput: float32(1.1)},
		{Script: `b()`, Input: map[string]interface{}{"b": func() float64 { return 1.1 }}, RunOutput: 1.1},
		{Script: `b()`, Input: map[string]interface{}{"b": func() string { return "a" }}, RunOutput: "a"},

		{Script: `b(a)`, Input: map[string]interface{}{"a": reflect.Value{}, "b": func(c reflect.Value) reflect.Value { return c }}, RunOutput: reflect.Value{}, Output: map[string]interface{}{"a": reflect.Value{}}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": nil, "b": func(c interface{}) interface{} { return c }}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": true, "b": func(c bool) bool { return c }}, RunOutput: true, Output: map[string]interface{}{"a": true}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": int32(1), "b": func(c int32) int32 { return c }}, RunOutput: int32(1), Output: map[string]interface{}{"a": int32(1)}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": int64(1), "b": func(c int64) int64 { return c }}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": float32(1.1), "b": func(c float32) float32 { return c }}, RunOutput: float32(1.1), Output: map[string]interface{}{"a": float32(1.1)}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": 1.1, "b": func(c float64) float64 { return c }}, RunOutput: 1.1, Output: map[string]interface{}{"a": 1.1}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": "a", "b": func(c string) string { return c }}, RunOutput: "a", Output: map[string]interface{}{"a": "a"}},

		{Script: `b(a)`, Input: map[string]interface{}{"a": "a", "b": func(c bool) bool { return c }}, RunError: fmt.Errorf("function wants argument type bool but received type string"), Output: map[string]interface{}{"a": "a"}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": int64(1), "b": func(c int32) int32 { return c }}, RunOutput: int32(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": int32(1), "b": func(c int64) int64 { return c }}, RunOutput: int64(1), Output: map[string]interface{}{"a": int32(1)}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": 1.25, "b": func(c float32) float32 { return c }}, RunOutput: float32(1.25), Output: map[string]interface{}{"a": 1.25}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": float32(1.25), "b": func(c float64) float64 { return c }}, RunOutput: 1.25, Output: map[string]interface{}{"a": float32(1.25)}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": true, "b": func(c string) string { return c }}, RunError: fmt.Errorf("function wants argument type string but received type bool"), Output: map[string]interface{}{"a": true}},

		{Script: `b(a)`, Input: map[string]interface{}{"a": testVarValueBool, "b": func(c interface{}) interface{} { return c }}, RunOutput: testVarValueBool, Output: map[string]interface{}{"a": testVarValueBool}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": testVarValueInt32, "b": func(c interface{}) interface{} { return c }}, RunOutput: testVarValueInt32, Output: map[string]interface{}{"a": testVarValueInt32}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": testVarValueInt64, "b": func(c interface{}) interface{} { return c }}, RunOutput: testVarValueInt64, Output: map[string]interface{}{"a": testVarValueInt64}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": testVarValueFloat32, "b": func(c interface{}) interface{} { return c }}, RunOutput: testVarValueFloat32, Output: map[string]interface{}{"a": testVarValueFloat32}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": testVarValueFloat64, "b": func(c interface{}) interface{} { return c }}, RunOutput: testVarValueFloat64, Output: map[string]interface{}{"a": testVarValueFloat64}},
		{Script: `b(a)`, Input: map[string]interface{}{"a": testVarValueString, "b": func(c interface{}) interface{} { return c }}, RunOutput: testVarValueString, Output: map[string]interface{}{"a": testVarValueString}},

		{Script: `func aFunc() {}; aFunc()`, RunOutput: nil},
		{Script: `func aFunc() { return }; aFunc()`, RunOutput: nil},
		{Script: `func aFunc() { return }; a = aFunc()`, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `func aFunc() { return 1 }; aFunc()`, RunOutput: int64(1)},
		{Script: `func aFunc() { return 1 }; a = aFunc()`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},

		{Script: `func aFunc() {return nil}; aFunc()`, RunOutput: nil},
		{Script: `func aFunc() {return true}; aFunc()`, RunOutput: true},
		{Script: `func aFunc() {return 1}; aFunc()`, RunOutput: int64(1)},
		{Script: `func aFunc() {return 1.1}; aFunc()`, RunOutput: 1.1},
		{Script: `func aFunc() {return "a"}; aFunc()`, RunOutput: "a"},

		{Script: `func aFunc() {return 1 + 2}; aFunc()`, RunOutput: int64(3)},
		{Script: `func aFunc() {return 1.25 + 2.25}; aFunc()`, RunOutput: 3.5},
		{Script: `func aFunc() {return "a" + "b"}; aFunc()`, RunOutput: "ab"},

		{Script: `func aFunc() {return 1 + 2, 3 + 4}; aFunc()`, RunOutput: []interface{}{int64(3), int64(7)}},
		{Script: `func aFunc() {return 1.25 + 2.25, 3.25 + 4.25}; aFunc()`, RunOutput: []interface{}{3.5, 7.5}},
		{Script: `func aFunc() {return "a" + "b", "c" + "d"}; aFunc()`, RunOutput: []interface{}{"ab", "cd"}},

		{Script: `func aFunc() {return nil, nil}; aFunc()`, RunOutput: []interface{}{nil, nil}},
		{Script: `func aFunc() {return true, false}; aFunc()`, RunOutput: []interface{}{true, false}},
		{Script: `func aFunc() {return 1, 2}; aFunc()`, RunOutput: []interface{}{int64(1), int64(2)}},
		{Script: `func aFunc() {return 1.1, 2.2}; aFunc()`, RunOutput: []interface{}{1.1, 2.2}},
		{Script: `func aFunc() {return "a", "b"}; aFunc()`, RunOutput: []interface{}{"a", "b"}},

		{Script: `func aFunc() {return [nil]}; aFunc()`, RunOutput: []interface{}{nil}},
		{Script: `func aFunc() {return [nil, nil]}; aFunc()`, RunOutput: []interface{}{nil, nil}},
		{Script: `func aFunc() {return [nil, nil, nil]}; aFunc()`, RunOutput: []interface{}{nil, nil, nil}},
		{Script: `func aFunc() {return [nil, nil], [nil, nil]}; aFunc()`, RunOutput: []interface{}{[]interface{}{nil, nil}, []interface{}{nil, nil}}},

		{Script: `func aFunc() {return [true]}; aFunc()`, RunOutput: []interface{}{true}},
		{Script: `func aFunc() {return [true, false]}; aFunc()`, RunOutput: []interface{}{true, false}},
		{Script: `func aFunc() {return [true, false, true]}; aFunc()`, RunOutput: []interface{}{true, false, true}},
		{Script: `func aFunc() {return [true, false], [false, true]}; aFunc()`, RunOutput: []interface{}{[]interface{}{true, false}, []interface{}{false, true}}},

		{Script: `func aFunc() {return []}; aFunc()`, RunOutput: []interface{}{}},
		{Script: `func aFunc() {return [1]}; aFunc()`, RunOutput: []interface{}{int64(1)}},
		{Script: `func aFunc() {return [1, 2]}; aFunc()`, RunOutput: []interface{}{int64(1), int64(2)}},
		{Script: `func aFunc() {return [1, 2, 3]}; aFunc()`, RunOutput: []interface{}{int64(1), int64(2), int64(3)}},
		{Script: `func aFunc() {return [1, 2], [3, 4]}; aFunc()`, RunOutput: []interface{}{[]interface{}{int64(1), int64(2)}, []interface{}{int64(3), int64(4)}}},

		{Script: `func aFunc() {return [1.1]}; aFunc()`, RunOutput: []interface{}{1.1}},
		{Script: `func aFunc() {return [1.1, 2.2]}; aFunc()`, RunOutput: []interface{}{1.1, 2.2}},
		{Script: `func aFunc() {return [1.1, 2.2, 3.3]}; aFunc()`, RunOutput: []interface{}{1.1, 2.2, 3.3}},
		{Script: `func aFunc() {return [1.1, 2.2], [3.3, 4.4]}; aFunc()`, RunOutput: []interface{}{[]interface{}{1.1, 2.2}, []interface{}{3.3, 4.4}}},

		{Script: `func aFunc() {return ["a"]}; aFunc()`, RunOutput: []interface{}{"a"}},
		{Script: `func aFunc() {return ["a", "b"]}; aFunc()`, RunOutput: []interface{}{"a", "b"}},
		{Script: `func aFunc() {return ["a", "b", "c"]}; aFunc()`, RunOutput: []interface{}{"a", "b", "c"}},
		{Script: `func aFunc() {return ["a", "b"], ["c", "d"]}; aFunc()`, RunOutput: []interface{}{[]interface{}{"a", "b"}, []interface{}{"c", "d"}}},

		{Script: `func aFunc() {return nil, nil}; aFunc()`, RunOutput: []interface{}{interface{}(nil), interface{}(nil)}},
		{Script: `func aFunc() {return true, false}; aFunc()`, RunOutput: []interface{}{true, false}},
		{Script: `func aFunc() {return 1, 2}; aFunc()`, RunOutput: []interface{}{int64(1), int64(2)}},
		{Script: `func aFunc() {return 1.1, 2.2}; aFunc()`, RunOutput: []interface{}{1.1, 2.2}},
		{Script: `func aFunc() {return "a", "b"}; aFunc()`, RunOutput: []interface{}{"a", "b"}},

		{Script: `func aFunc() {return a}; aFunc()`, Input: map[string]interface{}{"a": reflect.Value{}}, RunOutput: reflect.Value{}, Output: map[string]interface{}{"a": reflect.Value{}}},

		{Script: `func aFunc() {return a}; aFunc()`, Input: map[string]interface{}{"a": nil}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `func aFunc() {return a}; aFunc()`, Input: map[string]interface{}{"a": true}, RunOutput: true, Output: map[string]interface{}{"a": true}},
		{Script: `func aFunc() {return a}; aFunc()`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `func aFunc() {return a}; aFunc()`, Input: map[string]interface{}{"a": 1.1}, RunOutput: 1.1, Output: map[string]interface{}{"a": 1.1}},
		{Script: `func aFunc() {return a}; aFunc()`, Input: map[string]interface{}{"a": "a"}, RunOutput: "a", Output: map[string]interface{}{"a": "a"}},

		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": reflect.Value{}}, RunOutput: []interface{}{reflect.Value{}, reflect.Value{}}, Output: map[string]interface{}{"a": reflect.Value{}}},
		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": nil}, RunOutput: []interface{}{nil, nil}, Output: map[string]interface{}{"a": nil}},
		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": true}, RunOutput: []interface{}{true, true}, Output: map[string]interface{}{"a": true}},
		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": int32(1)}, RunOutput: []interface{}{int32(1), int32(1)}, Output: map[string]interface{}{"a": int32(1)}},
		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: []interface{}{int64(1), int64(1)}, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": float32(1.1)}, RunOutput: []interface{}{float32(1.1), float32(1.1)}, Output: map[string]interface{}{"a": float32(1.1)}},
		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": 1.1}, RunOutput: []interface{}{1.1, 1.1}, Output: map[string]interface{}{"a": 1.1}},
		{Script: `func aFunc() {return a, a}; aFunc()`, Input: map[string]interface{}{"a": "a"}, RunOutput: []interface{}{"a", "a"}, Output: map[string]interface{}{"a": "a"}},

		{Script: `func a(x) { return x}; a(nil)`, RunOutput: nil},
		{Script: `func a(x) { return x}; a(true)`, RunOutput: true},
		{Script: `func a(x) { return x}; a(1)`, RunOutput: int64(1)},
		{Script: `func a(x) { return x}; a(1.1)`, RunOutput: 1.1},
		{Script: `func a(x) { return x}; a("a")`, RunOutput: "a"},

		{Script: `func aFunc() {return a}; for {aFunc(); break}`, Input: map[string]interface{}{"a": nil}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `func aFunc() {return a}; for {aFunc(); break}`, Input: map[string]interface{}{"a": true}, RunOutput: nil, Output: map[string]interface{}{"a": true}},
		{Script: `func aFunc() {return a}; for {aFunc(); break}`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `func aFunc() {return a}; for {aFunc(); break}`, Input: map[string]interface{}{"a": 1.1}, RunOutput: nil, Output: map[string]interface{}{"a": 1.1}},
		{Script: `func aFunc() {return a}; for {aFunc(); break}`, Input: map[string]interface{}{"a": "a"}, RunOutput: nil, Output: map[string]interface{}{"a": "a"}},

		{Script: `func aFunc() {for {return a}}; aFunc()`, Input: map[string]interface{}{"a": nil}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `func aFunc() {for {return a}}; aFunc()`, Input: map[string]interface{}{"a": true}, RunOutput: true, Output: map[string]interface{}{"a": true}},
		{Script: `func aFunc() {for {return a}}; aFunc()`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `func aFunc() {for {return a}}; aFunc()`, Input: map[string]interface{}{"a": 1.1}, RunOutput: 1.1, Output: map[string]interface{}{"a": 1.1}},
		{Script: `func aFunc() {for {return a}}; aFunc()`, Input: map[string]interface{}{"a": "a"}, RunOutput: "a", Output: map[string]interface{}{"a": "a"}},

		{Script: `func aFunc() {for {if true {return a}}}; aFunc()`, Input: map[string]interface{}{"a": nil}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `func aFunc() {for {if true {return a}}}; aFunc()`, Input: map[string]interface{}{"a": true}, RunOutput: true, Output: map[string]interface{}{"a": true}},
		{Script: `func aFunc() {for {if true {return a}}}; aFunc()`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `func aFunc() {for {if true {return a}}}; aFunc()`, Input: map[string]interface{}{"a": 1.1}, RunOutput: 1.1, Output: map[string]interface{}{"a": 1.1}},
		{Script: `func aFunc() {for {if true {return a}}}; aFunc()`, Input: map[string]interface{}{"a": "a"}, RunOutput: "a", Output: map[string]interface{}{"a": "a"}},

		{Script: `func aFunc() {return nil, nil}; a, b = aFunc()`, RunOutput: nil, Output: map[string]interface{}{"a": nil, "b": nil}},
		{Script: `func aFunc() {return true, false}; a, b = aFunc()`, RunOutput: false, Output: map[string]interface{}{"a": true, "b": false}},
		{Script: `func aFunc() {return 1, 2}; a, b = aFunc()`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(1), "b": int64(2)}},
		{Script: `func aFunc() {return 1.1, 2.2}; a, b = aFunc()`, RunOutput: 2.2, Output: map[string]interface{}{"a": 1.1, "b": 2.2}},
		{Script: `func aFunc() {return "a", "b"}; a, b = aFunc()`, RunOutput: "b", Output: map[string]interface{}{"a": "a", "b": "b"}},
	}
	runTests(t, tests, nil, &Options{Debug: true})
}

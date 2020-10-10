package vm

import (
	"fmt"
	"reflect"
	"testing"
)

func TestIf(t *testing.T) {
	tests := []*Test{
		{Script: `if 1++ {}`, RunError: fmt.Errorf("invalid operation")},
		{Script: `if false {} else if 1++ {}`, RunError: fmt.Errorf("invalid operation")},
		{Script: `if false {} else if true { 1++ }`, RunError: fmt.Errorf("invalid operation")},

		{Script: `if true {}`, Input: map[string]interface{}{"a": nil}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `if true {}`, Input: map[string]interface{}{"a": true}, RunOutput: nil, Output: map[string]interface{}{"a": true}},
		{Script: `if true {}`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `if true {}`, Input: map[string]interface{}{"a": 1.1}, RunOutput: nil, Output: map[string]interface{}{"a": 1.1}},
		{Script: `if true {}`, Input: map[string]interface{}{"a": "a"}, RunOutput: nil, Output: map[string]interface{}{"a": "a"}},

		{Script: `if true {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `if true {a = true}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": true}},
		{Script: `if true {a = 1}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `if true {a = 1.1}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: 1.1, Output: map[string]interface{}{"a": 1.1}},
		{Script: `if true {a = "a"}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: "a", Output: map[string]interface{}{"a": "a"}},

		{Script: `if a == 1 {a = 1}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `if a == 2 {a = 1}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `if a == 1 {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `if a == 2 {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},

		{Script: `if a == 1 {a = 1} else {a = 3}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(3), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `if a == 2 {a = 1} else {a = 3}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `if a == 1 {a = 1} else if a == 3 {a = 3}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `if a == 1 {a = 1} else if a == 2 {a = 3}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(3), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `if a == 1 {a = 1} else if a == 3 {a = 3} else {a = 4}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(4), Output: map[string]interface{}{"a": int64(4)}},

		{Script: `if a == 1 {a = 1} else {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `if a == 2 {a = nil} else {a = 3}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `if a == 1 {a = nil} else if a == 3 {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `if a == 1 {a = 1} else if a == 2 {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `if a == 1 {a = 1} else if a == 3 {a = 3} else {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},

		{Script: `if a == 1 {a = 1} else if a == 3 {a = 3} else if a == 4 {a = 4} else {a = 5}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(5)}},
		{Script: `if a == 1 {a = 1} else if a == 3 {a = 3} else if a == 4 {a = 4} else {a = nil}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},
		{Script: `if a == 1 {a = 1} else if a == 3 {a = 3} else if a == 2 {a = 4} else {a = 5}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(4), Output: map[string]interface{}{"a": int64(4)}},
		{Script: `if a == 1 {a = 1} else if a == 3 {a = 3} else if a == 2 {a = nil} else {a = 5}`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: nil, Output: map[string]interface{}{"a": nil}},

		// check scope
		{Script: `a = 1; if a == 1 { b = 2 }; b`, RunError: fmt.Errorf("undefined symbol \"b\""), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; if a == 2 { b = 3 } else { b = 4 }; b`, RunError: fmt.Errorf("undefined symbol \"b\""), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; if a == 2 { b = 3 } else if a == 1 { b = 4 }; b`, RunError: fmt.Errorf("undefined symbol \"b\""), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; if a == 2 { b = 4 } else if a == 5 { b = 6 } else if a == 1 { c = b }`, RunError: fmt.Errorf("undefined symbol \"b\""), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; if a == 2 { b = 4 } else if a == 5 { b = 6 } else if a == 1 { b = 7 }; b`, RunError: fmt.Errorf("undefined symbol \"b\""), Output: map[string]interface{}{"a": int64(1)}},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestForLoop(t *testing.T) {
	tests := []*Test{
		{Script: `for in [1] { }`, ParseError: fmt.Errorf("missing identifier")},
		{Script: `for a, b, c in [1] { }`, ParseError: fmt.Errorf("too many identifiers")},

		{Script: `break`, RunError: fmt.Errorf("unexpected break statement")},
		{Script: `continue`, RunError: fmt.Errorf("unexpected continue statement")},
		{Script: `for 1++ { }`, RunError: fmt.Errorf("invalid operation")},
		{Script: `for { 1++ }`, RunError: fmt.Errorf("invalid operation")},
		{Script: `for a in 1++ { }`, RunError: fmt.Errorf("invalid operation")},

		{Script: `for { break }`, RunOutput: nil},
		{Script: `for {a = 1; if a == 1 { break } }`, RunOutput: nil},
		{Script: `a = 1; for { if a == 1 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for { if a == 1 { break }; a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for { if a == 3 { break }; a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},

		{Script: `a = 1; for { if a == 1 { return }; a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for { if a == 3 { return }; a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 1; for { if a == 1 { return 2 }; a++ }`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for { if a == 3 { return 2 }; a++ }`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(3)}},

		{Script: `a = 1; for { if a == 3 { return 3 }; a++ }; return 2`, RunOutput: int64(3), Output: map[string]interface{}{"a": int64(3)}},

		{Script: `a = 1; for { a++; if a == 2 { continue } else { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 1; for { a++; if a == 2 { continue }; if a == 3 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},

		{Script: `for a in [1] { if a == 1 { break } }`, RunOutput: nil},
		{Script: `for a in [1, 2] { if a == 2 { break } }`, RunOutput: nil},
		{Script: `for a in [1, 2, 3] { if a == 3 { break } }`, RunOutput: nil},

		{Script: `for a in [1, 2, 3] { if a == 2 { return 2 } }; return 3`, RunOutput: int64(2)},

		{Script: `a = [1]; for b in a { if b == 1 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int64(1)}}},
		{Script: `a = [1, 2]; for b in a { if b == 2 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int64(1), int64(2)}}},
		{Script: `a = [1, 2, 3]; for b in a { if b == 3 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int64(1), int64(2), int64(3)}}},

		{Script: `a = [1]; b = 0; for c in a { b = c }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int64(1)}, "b": int64(1)}},
		{Script: `a = [1, 2]; b = 0; for c in a { b = c }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int64(1), int64(2)}, "b": int64(2)}},
		{Script: `a = [1, 2, 3]; b = 0; for c in a { b = c }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int64(1), int64(2), int64(3)}, "b": int64(3)}},

		{Script: `a = 1; for a < 2 { a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; for a < 3 { a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},

		{Script: `a = 1; for nil { a++; if a > 2 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for nil { a++; if a > 3 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for true { a++; if a > 2 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 1; for true { a++; if a > 3 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(4)}},

		{Script: `func x() { return [1] }; for b in x() { if b == 1 { break } }`, RunOutput: nil},
		{Script: `func x() { return [1, 2] }; for b in x() { if b == 2 { break } }`, RunOutput: nil},
		{Script: `func x() { return [1, 2, 3] }; for b in x() { if b == 3 { break } }`, RunOutput: nil},

		{Script: `func x() { a = 1; for { if a == 1 { return } } }; x()`, RunOutput: nil},
		{Script: `func x() { a = 1; for { if a == 1 { return nil } } }; x()`, RunOutput: nil},
		{Script: `func x() { a = 1; for { if a == 1 { return true } } }; x()`, RunOutput: true},
		{Script: `func x() { a = 1; for { if a == 1 { return 1 } } }; x()`, RunOutput: int64(1)},
		{Script: `func x() { a = 1; for { if a == 1 { return 1.1 } } }; x()`, RunOutput: 1.1},
		{Script: `func x() { a = 1; for { if a == 1 { return "a" } } }; x()`, RunOutput: "a"},

		{Script: `func x() { for a in [1, 2, 3] { if a == 3 { return } } }; x()`, RunOutput: nil},
		{Script: `func x() { for a in [1, 2, 3] { if a == 3 { return 3 } }; return 2 }; x()`, RunOutput: int64(3)},
		{Script: `func x() { for a in [1, 2, 3] { if a == 1 { continue } } }; x()`, RunOutput: nil},
		{Script: `func x() { for a in [1, 2, 3] { if a == 1 { continue };  if a == 3 { return } } }; x()`, RunOutput: nil},

		{Script: `func x() { return [1, 2] }; a = 1; for i in x() { a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},
		{Script: `func x() { return [1, 2, 3] }; a = 1; for i in x() { a++ }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(4)}},

		{Script: `for a = 1; nil; nil { return }`},
		// TO FIX:
		// {Script: `for a, b = 1; nil; nil { return }`},
		// {Script: `for a, b = 1, 2; nil; nil { return }`},

		{Script: `var a = 1; for ; ; { return a }`, RunOutput: int64(1)},
		{Script: `var a = 1; for ; ; a++ { return a }`, RunOutput: int64(1)},
		{Script: `var a = 1; for ; a > 0 ; { return a }`, RunOutput: int64(1)},
		{Script: `var a = 1; for ; a > 0 ; a++ { return a }`, RunOutput: int64(1)},
		{Script: `for var a = 1 ; ; { return a }`, RunOutput: int64(1)},
		{Script: `for var a = 1 ; ; a++ { return a }`, RunOutput: int64(1)},
		{Script: `for var a = 1 ; a > 0 ; { return a }`, RunOutput: int64(1)},
		{Script: `for var a = 1 ; a > 0 ; a++ { return a }`, RunOutput: int64(1)},

		{Script: `for var a = 1; nil; nil { return }`},
		{Script: `for var a = 1, 2; nil; nil { return }`},
		{Script: `for var a, b = 1; nil; nil { return }`},
		{Script: `for var a, b = 1, 2; nil; nil { return }`},

		{Script: `for a.b = 1; nil; nil { return }`, RunError: fmt.Errorf("undefined symbol \"a\"")},

		{Script: `for a = 1; nil; nil { if a == 1 { break } }`, RunOutput: nil},
		{Script: `for a = 1; nil; nil { if a == 2 { break }; a++ }`, RunOutput: nil},
		{Script: `for a = 1; nil; nil { a++; if a == 3 { break } }`, RunOutput: nil},

		{Script: `for a = 1; a < 1; nil { }`, RunOutput: nil},
		{Script: `for a = 1; a > 1; nil { }`, RunOutput: nil},
		{Script: `for a = 1; a == 1; nil { break }`, RunOutput: nil},

		{Script: `for a = 1; a == 1; a++ { }`, RunOutput: nil},
		{Script: `for a = 1; a < 2; a++ { }`, RunOutput: nil},
		{Script: `for a = 1; a < 3; a++ { }`, RunOutput: nil},

		{Script: `for a = 1; a < 5; a++ { if a == 3 { return 3 } }; return 2`, RunOutput: int64(3)},

		{Script: `a = 1; for b = 1; a < 1; a++ { }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for b = 1; a < 2; a++ { }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; for b = 1; a < 3; a++ { }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},

		{Script: `a = 1; for b = 1; a < 1; a++ {  if a == 1 { continue } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for b = 1; a < 2; a++ {  if a == 1 { continue } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; for b = 1; a < 3; a++ {  if a == 1 { continue } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},

		{Script: `a = 1; for b = 1; a < 1; a++ {  if a == 1 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for b = 1; a < 2; a++ {  if a == 1 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for b = 1; a < 3; a++ {  if a == 1 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for b = 1; a < 1; a++ {  if a == 2 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for b = 1; a < 2; a++ {  if a == 2 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; for b = 1; a < 3; a++ {  if a == 2 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; for b = 1; a < 1; a++ {  if a == 3 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; for b = 1; a < 2; a++ {  if a == 3 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; for b = 1; a < 3; a++ {  if a == 3 { break } }`, RunOutput: nil, Output: map[string]interface{}{"a": int64(3)}},

		{Script: `a = ["123", "456", "789"]; b = ""; for i = 0; i < len(a); i++ { b += a[i][len(a[i]) - 2:]; b += a[i][:len(a[i]) - 2] }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{"123", "456", "789"}, "b": "231564897"}},
		{Script: `a = [[["123"], ["456"]], [["789"]]]; b = ""; for i = 0; i < len(a); i++ { for j = 0; j < len(a[i]); j++ {  for k = 0; k < len(a[i][j]); k++ { for l = 0; l < len(a[i][j][k]); l++ { b += a[i][j][k][l] + "-" } } } }`,
			RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{[]interface{}{[]interface{}{"123"}, []interface{}{"456"}}, []interface{}{[]interface{}{"789"}}}, "b": "1-2-3-4-5-6-7-8-9-"}},

		{Script: `func x() { for a = 1; a < 3; a++ { if a == 1 { return a } } }; x()`, RunOutput: int64(1)},
		{Script: `func x() { for a = 1; a < 3; a++ { if a == 2 { return a } } }; x()`, RunOutput: int64(2)},
		{Script: `func x() { for a = 1; a < 3; a++ { if a == 3 { return a } } }; x()`, RunOutput: nil},
		{Script: `func x() { for a = 1; a < 3; a++ { if a == 4 { return a } } }; x()`, RunOutput: nil},

		{Script: `func x() { a = 1; for b = 1; a < 3; a++ { if a == 1 { continue } }; return a }; x()`, RunOutput: int64(3)},
		{Script: `func x() { a = 1; for b = 1; a < 3; a++ { if a == 2 { continue } }; return a }; x()`, RunOutput: int64(3)},
		{Script: `func x() { a = 1; for b = 1; a < 3; a++ { if a == 3 { continue } }; return a }; x()`, RunOutput: int64(3)},
		{Script: `func x() { a = 1; for b = 1; a < 3; a++ { if a == 4 { continue } }; return a }; x()`, RunOutput: int64(3)},

		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{reflect.Value{}}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{reflect.Value{}}, "b": reflect.Value{}}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{nil}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{nil}, "b": nil}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{true}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{true}, "b": true}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{int32(1)}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int32(1)}, "b": int32(1)}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{int64(1)}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{int64(1)}, "b": int64(1)}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{float32(1.1)}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{float32(1.1)}, "b": float32(1.1)}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{1.1}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{1.1}, "b": 1.1}},

		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{interface{}(reflect.Value{})}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{interface{}(reflect.Value{})}, "b": interface{}(reflect.Value{})}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{interface{}(nil)}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{interface{}(nil)}, "b": interface{}(nil)}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{interface{}(true)}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{interface{}(true)}, "b": interface{}(true)}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{interface{}(int32(1))}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{interface{}(int32(1))}, "b": interface{}(int32(1))}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{interface{}(int64(1))}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{interface{}(int64(1))}, "b": interface{}(int64(1))}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{interface{}(float32(1.1))}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{interface{}(float32(1.1))}, "b": interface{}(float32(1.1))}},
		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": []interface{}{interface{}(1.1)}}, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{interface{}(1.1)}, "b": interface{}(1.1)}},

		{Script: `b = 0; for i in a { b = i }`, Input: map[string]interface{}{"a": interface{}([]interface{}{nil})}, RunOutput: nil, Output: map[string]interface{}{"a": interface{}([]interface{}{nil}), "b": nil}},

		{Script: `for i in nil { }`, RunError: fmt.Errorf("for cannot loop over type interface")},
		{Script: `for i in true { }`, RunError: fmt.Errorf("for cannot loop over type bool")},
		{Script: `for i in a { }`, Input: map[string]interface{}{"a": reflect.Value{}}, RunError: fmt.Errorf("for cannot loop over type struct"), Output: map[string]interface{}{"a": reflect.Value{}}},
		{Script: `for i in a { }`, Input: map[string]interface{}{"a": interface{}(nil)}, RunError: fmt.Errorf("for cannot loop over type interface"), Output: map[string]interface{}{"a": interface{}(nil)}},
		{Script: `for i in a { }`, Input: map[string]interface{}{"a": interface{}(true)}, RunError: fmt.Errorf("for cannot loop over type bool"), Output: map[string]interface{}{"a": interface{}(true)}},
		{Script: `for i in [1, 2, 3] { b++ }`, RunError: fmt.Errorf("undefined symbol \"b\"")},
		{Script: `for a = 1; a < 3; a++ { b++ }`, RunError: fmt.Errorf("undefined symbol \"b\"")},
		{Script: `for a = b; a < 3; a++ { }`, RunError: fmt.Errorf("undefined symbol \"b\"")},
		{Script: `for a = 1; b < 3; a++ { }`, RunError: fmt.Errorf("undefined symbol \"b\"")},
		{Script: `for a = 1; a < 3; b++ { }`, RunError: fmt.Errorf("undefined symbol \"b\"")},

		{Script: `a = 1; b = [{"c": "c"}]; for i in b { a = i }`, RunOutput: nil, Output: map[string]interface{}{"a": map[interface{}]interface{}{"c": "c"}, "b": []interface{}{map[interface{}]interface{}{"c": "c"}}}},
		{Script: `a = 1; b = {"x": [{"y": "y"}]};  for i in b.x { a = i }`, RunOutput: nil, Output: map[string]interface{}{"a": map[interface{}]interface{}{"y": "y"}, "b": map[interface{}]interface{}{"x": []interface{}{map[interface{}]interface{}{"y": "y"}}}}},

		{Script: `a = {}; b = 1; for i in a { b = i }; b`, RunOutput: int64(1), Output: map[string]interface{}{"a": map[interface{}]interface{}{}, "b": int64(1)}},
		{Script: `a = {"x": 2}; b = 1; for i in a { b = i }; b`, RunOutput: "x", Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2)}, "b": "x"}},
		{Script: `a = {"x": 2, "y": 3}; b = 0; for i in a { b++ }; b`, RunOutput: int64(2), Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}, "b": int64(2)}},
		{Script: `a = {"x": 2, "y": 3}; for i in a { b++ }`, RunError: fmt.Errorf("undefined symbol \"b\""), Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}}},

		{Script: `a = {"x": 2, "y": 3}; b = 0; for i in a { if i ==  "x" { continue }; b = i }; b`, RunOutput: "y", Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}, "b": "y"}},
		{Script: `a = {"x": 2, "y": 3}; b = 0; for i in a { if i ==  "y" { continue }; b = i }; b`, RunOutput: "x", Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}, "b": "x"}},
		{Script: `a = {"x": 2, "y": 3}; for i in a { if i ==  "x" { return 1 } }`, RunOutput: int64(1), Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}}},
		{Script: `a = {"x": 2, "y": 3}; for i in a { if i ==  "y" { return 2 } }`, RunOutput: int64(2), Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}}},
		{Script: `a = {"x": 2, "y": 3}; b = 0; for i in a { if i ==  "x" { break }; b++ }; if b > 1 { return false } else { return true }`, RunOutput: true, Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}}},
		{Script: `a = {"x": 2, "y": 3}; b = 0; for i in a { if i ==  "y" { break }; b++ }; if b > 1 { return false } else { return true }`, RunOutput: true, Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}}},
		{Script: `a = {"x": 2, "y": 3}; b = 1; for i in a { if (i ==  "x" || i ==  "y") { break }; b++ }; b`, RunOutput: int64(1), Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2), "y": int64(3)}, "b": int64(1)}},

		{Script: `a = ["123", "456", "789"]; b = ""; for v in a { b += v[len(v) - 2:]; b += v[:len(v) - 2] }`, RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{"123", "456", "789"}, "b": "231564897"}},
		{Script: `a = [[["123"], ["456"]], [["789"]]]; b = ""; for x in a { for y in x  {  for z in y { for i = 0; i < len(z); i++ { b += z[i] + "-" } } } }`,
			RunOutput: nil, Output: map[string]interface{}{"a": []interface{}{[]interface{}{[]interface{}{"123"}, []interface{}{"456"}}, []interface{}{[]interface{}{"789"}}}, "b": "1-2-3-4-5-6-7-8-9-"}},

		{Script: `a = {"x": 2}; b = 0; for k, v in a { b = k }; b`, RunOutput: "x", Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2)}, "b": "x"}},
		{Script: `a = {"x": 2}; b = 0; for k, v in a { b = v }; b`, RunOutput: int64(2), Output: map[string]interface{}{"a": map[interface{}]interface{}{"x": int64(2)}, "b": int64(2)}},

		{Script: `a = make(chan int64, 2); a <- 1; v = 0; for val in a { v = val; break; }; v`, RunOutput: int64(1), Output: map[string]interface{}{"v": int64(1)}},
		{Script: `a = make(chan int64, 4); a <- 1; a <- 2; a <- 3; for i in a { if i == 2 { return 2 } }; return 4`, RunOutput: int64(2)},
		{Script: `a = make(chan int64, 2); a <- 1; for i in a { if i < 4 { a <- i + 1; continue }; return 4 }; return 6`, RunOutput: int64(4)},

		// test non-buffer and go func
		{Script: `a = make(chan int64); go func() { a <- 1; a <- 2; a <- 3 }(); b = []; for i in a { b += i; if i > 2 { break } }`, RunOutput: nil, Output: map[string]interface{}{"b": []interface{}{int64(1), int64(2), int64(3)}}},
		{Script: `a = make(chan int64); go func() { a <- 1; a <- 2; a <- 3; close(a) }(); b = []; for i in a { b += i }`, RunOutput: nil, Output: map[string]interface{}{"b": []interface{}{int64(1), int64(2), int64(3)}}},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestSelectSend(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()
	// left channel is first
	a := make(chan int64, 1)
	// b := make(chan int64, 1)
	csd := func() chan int64 {
		return a
	}
	select {
	case csd() <- 1: // invalid b type
	}

	tests := []*Test{
		// {Script: `a = make(chan int64, 1); a <- 123 ; vv = 1; ok = false; select {case  vv, ok = <- a: return vv, ok}`, RunOutput: int64(1)},

		// test send 1 channel
		{Script: `a = make(chan int64, 1); b = map[int64]int64{1:1  } ; vv = 1; select {case <- b[1]: return 1}`, RunOutput: int64(1)},
		// {Script: `a = make(chan int64, 1); b = make(chan int64, 1);  b<- 1 ; a <- <- b; <-a`, RunOutput: int64(1)},
		// {Script: `a = func(){return make(chan int64, 1)}; a() <- 1; return 1`, RunOutput: int64(1)},

		// {Script: `a = make(chan int64, 1); vv = 1; select {case a <- 1: return 1}`, RunOutput: int64(1)},

		// test send 2 channels
		// {Script: `a = make(chan int64, 1); vv = 1; select {case a <- vv: return 1}`, RunOutput: int64(1)},
		// {Script: `a = make(chan int64, 1); vv = 1; select {case a <- vv: return 1}`, RunOutput: int64(1)},

		// default
	}
	runTests(t, tests, &Options{Debug: false})
}

func TestSelect(t *testing.T) {
	tests := []*Test{
		// test parse errors
		{Script: `select {default: return 6; default: return 7}`, ParseError: fmt.Errorf("multiple default statement"), RunOutput: int64(7)},
		{Script: `a = make(chan int64, 1); a <- 1; select {case <-a: return 5; default: return 6; default: return 7}`, ParseError: fmt.Errorf("multiple default statement"), RunOutput: int64(5)},
		{Script: `select {case:}`, ParseError: fmt.Errorf("syntax error: unexpected ':'"), RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `select {case: return 1}`, ParseError: fmt.Errorf("syntax error: unexpected ':'"), RunError: fmt.Errorf("invalid operation"), RunOutput: nil},

		// test run errors
		{Script: `select {case a = <-b: return 1}`, RunError: fmt.Errorf("undefined symbol \"b\""), RunOutput: nil},
		{Script: `select {case b = 1: return 1}`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `select {case 1: return 1}`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `select {case <-1: return 1}`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `select {case <-a: return 1}`, RunError: fmt.Errorf("undefined symbol \"a\""), RunOutput: nil},
		{Script: `select {case if true { }: return 1}`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `a = make(chan int64, 1); a <- 1; select {case b.c = <-a: return 1}`, RunError: fmt.Errorf("undefined symbol \"b\""), RunOutput: nil},
		{Script: `a = make(chan int64, 1); a <- 1; select {case v = <-a:}; v`, RunError: fmt.Errorf("undefined symbol \"v\""), RunOutput: nil},

		// test receive 1 channel
		{Script: `a = make(chan int64, 1); a <- 1; select {case <-a: return 1}`, RunOutput: int64(1)},

		// test receive 2 channels
		{Script: `a = make(chan int64, 1); b = make(chan int64, 1); a <- 1; select {case <-a: return 1; case <-b: return 2}`, RunOutput: int64(1)},
		{Script: `a = make(chan int64, 1); b = make(chan int64, 1); b <- 1; select {case <-a: return 1; case <-b: return 2}`, RunOutput: int64(2)},

		// test default
		{Script: `select {default: return 1}`, RunOutput: int64(1)},

		{Script: `select {default:}; return 1`, RunOutput: int64(1)},
		{Script: `a = make(chan int64, 1); a <- 1; select {case <-a:}; return 1`, RunOutput: int64(1)},

		{Script: `a = make(chan int64, 1); b = make(chan int64, 1); select {case <-a: return 1; case <-b: return 2; default: return 3}`, RunOutput: int64(3)},
		{Script: `a = make(chan int64, 1); b = make(chan int64, 1); a <- 1; select {case <-a: return 1; case <-b: return 2; default: return 3}`, RunOutput: int64(1)},
		{Script: `a = make(chan int64, 1); b = make(chan int64, 1); b <- 1; select {case <-a: return 1; case <-b: return 2; default: return 3}`, RunOutput: int64(2)},

		// test assignment
		{Script: `a = make(chan int64, 1); b = make(chan int64, 1); a <- 1; v = 0; select {case v = <-a:; case v = <-b:}; v`, RunOutput: int64(1), Output: map[string]interface{}{"v": int64(1)}},
		{Script: `a = make(chan int64, 1); a <- 1; select {case v = <-a: return v}`, RunOutput: int64(1), Output: map[string]interface{}{}},

		// test new lines
		{Script: `
		a = make(chan int64, 1)
		a <- 1
		select {
			case <-a:
				return 1
		}`, RunOutput: int64(1)},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestSwitch(t *testing.T) {
	tests := []*Test{
		// test parse errors
		{Script: `switch {}`, ParseError: fmt.Errorf("syntax error")},
		{Script: `a = 1; switch a; {}`, ParseError: fmt.Errorf("syntax error")},
		{Script: `a = 1; switch a = 2 {}`, ParseError: fmt.Errorf("syntax error")},
		{Script: `a = 1; switch a {default: return 6; default: return 7}`, ParseError: fmt.Errorf("multiple default statement"), RunOutput: int64(7)},
		{Script: `a = 1; switch a {case 1: return 5; default: return 6; default: return 7}`, ParseError: fmt.Errorf("multiple default statement"), RunOutput: int64(5)},

		// test run errors
		{Script: `a = 1; switch 1++ {}`, RunError: fmt.Errorf("invalid operation")},
		{Script: `a = 1; switch a {case 1++: return 2}`, RunError: fmt.Errorf("invalid operation")},

		// test no or empty cases
		{Script: `a = 1; switch a {}`, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; switch a {case: return 2}`, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; switch a {case: return 2; case: return 3}`, Output: map[string]interface{}{"a": int64(1)}},

		// test 1 case
		{Script: `a = 1; switch a {case 1: return 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: return 5}`, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; switch a {case 1,2: return 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1,2: return 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1,2: return 5}`, Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 1; switch a {case 1,2,3: return 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1,2,3: return 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1,2,3: return 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 4; switch a {case 1,2,3: return 5}`, Output: map[string]interface{}{"a": int64(4)}},
		{Script: `a = func() { return 1 }; switch a() {case 1: return 5}`, RunOutput: int64(5)},
		{Script: `a = func() { return 2 }; switch a() {case 1: return 5}`},
		{Script: `a = func() { return 5 }; b = 1; switch b {case 1: return a() }`, RunOutput: int64(5), Output: map[string]interface{}{"b": int64(1)}},
		{Script: `a = func() { return 6 }; b = 2; switch b {case 1: return a() }`, Output: map[string]interface{}{"b": int64(2)}},

		// test 2 cases
		{Script: `a = 1; switch a {case 1: return 5; case 2: return 6}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: return 5; case 2: return 6}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1: return 5; case 2: return 6}`, Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 1; switch a {case 1: return 5; case 2,3: return 6}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: return 5; case 2,3: return 6}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1: return 5; case 2,3: return 6}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 4; switch a {case 1: return 5; case 2,3: return 6}`, Output: map[string]interface{}{"a": int64(4)}},

		// test 3 cases
		{Script: `a = 1; switch a {case 1,2: return 5; case 3: return 6}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1,2: return 5; case 3: return 6}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1,2: return 5; case 3: return 6}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 4; switch a {case 1,2: return 5; case 3: return 6}`, Output: map[string]interface{}{"a": int64(4)}},
		{Script: `a = 1; switch a {case 1,2: return 5; case 2,3: return 6}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1,2: return 5; case 2,3: return 6}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1,2: return 5; case 2,3: return 6}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 4; switch a {case 1,2: return 5; case 2,3: return 6}`, Output: map[string]interface{}{"a": int64(4)}},

		// test default
		{Script: `a = 1; switch a {default: return 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 1; switch a {case 1: return 5; case 2: return 6; default: return 7}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: return 5; case 2: return 6; default: return 7}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1: return 5; case 2: return 6; default: return 7}`, RunOutput: int64(7), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 1; switch a {case 1: return 5; case 2,3: return 6; default: return 7}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: return 5; case 2,3: return 6; default: return 7}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1: return 5; case 2,3: return 6; default: return 7}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 4; switch a {case 1: return 5; case 2,3: return 6; default: return 7}`, RunOutput: int64(7), Output: map[string]interface{}{"a": int64(4)}},
		{Script: `a = 1; switch a {case 1,2: return 5; case 3: return 6; default: return 7}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1,2: return 5; case 3: return 6; default: return 7}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 3; switch a {case 1,2: return 5; case 3: return 6; default: return 7}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a = 4; switch a {case 1,2: return 5; case 3: return 6; default: return 7}`, RunOutput: int64(7), Output: map[string]interface{}{"a": int64(4)}},

		// test scope
		{Script: `a = 1; switch a {case 1: a = 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(5)}},
		{Script: `a = 2; switch a {case 1: a = 5}`, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; b = 5; switch a {case 1: b = 6}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(1), "b": int64(6)}},
		{Script: `a = 2; b = 5; switch a {case 1: b = 6}`, Output: map[string]interface{}{"a": int64(2), "b": int64(5)}},
		{Script: `a = 1; b = 5; switch a {case 1: b = 6; default: b = 7}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(1), "b": int64(6)}},
		{Script: `a = 2; b = 5; switch a {case 1: b = 6; default: b = 7}`, RunOutput: int64(7), Output: map[string]interface{}{"a": int64(2), "b": int64(7)}},

		// test scope without define b
		{Script: `a = 1; switch a {case 1: b = 5}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: b = 5}`, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; switch a {case 1: b = 5}; b`, RunError: fmt.Errorf("undefined symbol \"b\""), RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: b = 5}; b`, RunError: fmt.Errorf("undefined symbol \"b\""), RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; switch a {case 1: b = 5; default: b = 6}`, RunOutput: int64(5), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: b = 5; default: b = 6}`, RunOutput: int64(6), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; switch a {case 1: b = 5; default: b = 6}; b`, RunError: fmt.Errorf("undefined symbol \"b\""), RunOutput: nil, Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2; switch a {case 1: b = 5; default: b = 6}; b`, RunError: fmt.Errorf("undefined symbol \"b\""), RunOutput: nil, Output: map[string]interface{}{"a": int64(2)}},

		// test new lines
		{Script: `
a = 1;
switch a {
	case 1:
		return 1
}`, RunOutput: int64(1)},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestTry(t *testing.T) {
	tests := []*Test{
		{Script: `try { 1++ } catch { 1++ }`, RunError: fmt.Errorf("invalid operation")},
		{Script: `try { 1++ } catch a { return a }`, RunOutput: fmt.Errorf("invalid operation")},
		{Script: `try { 1++ } catch a { a = 2 }; return a`, RunError: fmt.Errorf("undefined symbol \"a\"")},

		// test finally
		{Script: `try { 1++ } catch { 1++ } finally { return 1 }`, RunError: fmt.Errorf("invalid operation")},
		{Script: `try { } catch { } finally { 1++ }`, RunError: fmt.Errorf("invalid operation")},
		{Script: `try { } catch { 1 } finally { 1++ }`, RunError: fmt.Errorf("invalid operation")},
		{Script: `try { 1++ } catch { } finally { 1++ }`, RunError: fmt.Errorf("invalid operation")},
		{Script: `try { 1++ } catch a { } finally { return a }`, RunOutput: fmt.Errorf("invalid operation")},
		{Script: `try { 1++ } catch a { } finally { a = 2 }; return a`, RunError: fmt.Errorf("undefined symbol \"a\"")},

		{Script: `try { } catch { }`, RunOutput: nil},
		{Script: `try { 1++ } catch { }`, RunOutput: nil},
		{Script: `try { } catch { 1++ }`, RunOutput: nil},
		{Script: `try { return 1 } catch { }`, RunOutput: int64(1)},
		{Script: `try { return 1 } catch { return 2 }`, RunOutput: int64(2)},
		{Script: `try { 1++ } catch { return 1 }`, RunOutput: int64(1)},

		// test finally
		{Script: `try { } catch { } finally { return 1 }`, RunOutput: int64(1)},
		{Script: `try { 1++ } catch { } finally { return 1 }`, RunOutput: int64(1)},
		{Script: `try { 1++ } catch { return 1 } finally { 1++ }`, RunOutput: int64(1)},

		// test variable scope
		{Script: `try { 1++ } catch a { if a.Error() == "invalid operation" { return 1 } else { return 2 } }`, RunOutput: int64(1)},
		{Script: `try { 1++ } catch a { } finally { if a.Error() == "invalid operation" { return 1 } else { return 2 } }`, RunOutput: int64(1)},
	}
	runTests(t, tests, &Options{Debug: true})
}

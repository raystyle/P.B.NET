package vm

import (
	"fmt"
	"reflect"
	"testing"
)

func TestBasicOperators(t *testing.T) {
	tests := []*Test{
		{Script: `]`, ParseError: fmt.Errorf("syntax error")},

		{Script: `2 + 1`, RunOutput: int64(3)},
		{Script: `2 - 1`, RunOutput: int64(1)},
		{Script: `2 * 1`, RunOutput: int64(2)},
		{Script: `2 / 1`, RunOutput: float64(2)},
		{Script: `2.1 + 1.1`, RunOutput: 3.2},
		{Script: `2.1 - 1.1`, RunOutput: float64(1)},
		{Script: `2 + 1.1`, RunOutput: 3.1},
		{Script: `2.1 + 1`, RunOutput: 3.1},
		{Script: `3 - 1.5`, RunOutput: 1.5},
		{Script: `2.1 - 1`, RunOutput: 1.1},
		{Script: `2.1 * 2.0`, RunOutput: 4.2},
		{Script: `6.5 / 2.0`, RunOutput: 3.25},

		{Script: `2-1`, RunOutput: int64(1)},
		{Script: `2 -1`, RunOutput: int64(1)},
		{Script: `2- 1`, RunOutput: int64(1)},
		{Script: `2 - -1`, RunOutput: int64(3)},
		{Script: `2- -1`, RunOutput: int64(3)},
		{Script: `2 - - 1`, RunOutput: int64(3)},
		{Script: `2- - 1`, RunOutput: int64(3)},

		{Script: `a + b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: int64(3)},
		{Script: `a - b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: int64(1)},
		{Script: `a * b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: int64(2)},
		{Script: `a / b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: float64(2)},
		{Script: `a + b`, Input: map[string]interface{}{"a": 2.1, "b": 1.1}, RunOutput: 3.2},
		{Script: `a - b`, Input: map[string]interface{}{"a": 2.1, "b": 1.1}, RunOutput: float64(1)},
		{Script: `a * b`, Input: map[string]interface{}{"a": 2.1, "b": float64(2)}, RunOutput: 4.2},
		{Script: `a / b`, Input: map[string]interface{}{"a": 6.5, "b": float64(2)}, RunOutput: 3.25},

		{Script: `a + b`, Input: map[string]interface{}{"a": "a", "b": "b"}, RunOutput: "ab"},
		{Script: `a + b`, Input: map[string]interface{}{"a": "a", "b": int64(1)}, RunOutput: "a1"},
		{Script: `a + b`, Input: map[string]interface{}{"a": "a", "b": 1.1}, RunOutput: "a1.1"},
		{Script: `a + b`, Input: map[string]interface{}{"a": int64(2), "b": "b"}, RunOutput: "2b"},
		{Script: `a + b`, Input: map[string]interface{}{"a": 2.5, "b": "b"}, RunOutput: "2.5b"},

		{Script: `a + z`, Input: map[string]interface{}{"a": "a"}, RunError: fmt.Errorf("undefined symbol \"z\""), RunOutput: nil},
		{Script: `z + b`, Input: map[string]interface{}{"a": "a"}, RunError: fmt.Errorf("undefined symbol \"z\""), RunOutput: nil},

		{Script: `c = a + b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: int64(3), Output: map[string]interface{}{"c": int64(3)}},
		{Script: `c = a - b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: int64(1), Output: map[string]interface{}{"c": int64(1)}},
		{Script: `c = a * b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: int64(2), Output: map[string]interface{}{"c": int64(2)}},
		{Script: `c = a / b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: float64(2), Output: map[string]interface{}{"c": float64(2)}},
		{Script: `c = a + b`, Input: map[string]interface{}{"a": 2.1, "b": 1.1}, RunOutput: 3.2, Output: map[string]interface{}{"c": 3.2}},
		{Script: `c = a - b`, Input: map[string]interface{}{"a": 2.1, "b": 1.1}, RunOutput: float64(1), Output: map[string]interface{}{"c": float64(1)}},
		{Script: `c = a * b`, Input: map[string]interface{}{"a": 2.1, "b": float64(2)}, RunOutput: 4.2, Output: map[string]interface{}{"c": 4.2}},
		{Script: `c = a / b`, Input: map[string]interface{}{"a": 6.5, "b": float64(2)}, RunOutput: 3.25, Output: map[string]interface{}{"c": 3.25}},

		{Script: `a = nil; a++`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = false; a++`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = true; a++`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1; a++`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = 1.5; a++`, RunOutput: 2.5, Output: map[string]interface{}{"a": 2.5}},
		{Script: `a = "1"; a++`, RunOutput: "11", Output: map[string]interface{}{"a": "11"}},
		{Script: `a = "a"; a++`, RunOutput: "a1", Output: map[string]interface{}{"a": "a1"}},

		{Script: `a = nil; a--`, RunOutput: int64(-1), Output: map[string]interface{}{"a": int64(-1)}},
		{Script: `a = false; a--`, RunOutput: int64(-1), Output: map[string]interface{}{"a": int64(-1)}},
		{Script: `a = true; a--`, RunOutput: int64(0), Output: map[string]interface{}{"a": int64(0)}},
		{Script: `a = 2; a--`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = 2.5; a--`, RunOutput: 1.5, Output: map[string]interface{}{"a": 1.5}},

		{Script: `a++`, Input: map[string]interface{}{"a": nil}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a++`, Input: map[string]interface{}{"a": false}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a++`, Input: map[string]interface{}{"a": true}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a++`, Input: map[string]interface{}{"a": int32(1)}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a++`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a++`, Input: map[string]interface{}{"a": float32(3.5)}, RunOutput: 4.5, Output: map[string]interface{}{"a": 4.5}},
		{Script: `a++`, Input: map[string]interface{}{"a": 4.5}, RunOutput: 5.5, Output: map[string]interface{}{"a": 5.5}},
		{Script: `a++`, Input: map[string]interface{}{"a": "2"}, RunOutput: "21", Output: map[string]interface{}{"a": "21"}},
		{Script: `a++`, Input: map[string]interface{}{"a": "a"}, RunOutput: "a1", Output: map[string]interface{}{"a": "a1"}},

		{Script: `a--`, Input: map[string]interface{}{"a": nil}, RunOutput: int64(-1), Output: map[string]interface{}{"a": int64(-1)}},
		{Script: `a--`, Input: map[string]interface{}{"a": false}, RunOutput: int64(-1), Output: map[string]interface{}{"a": int64(-1)}},
		{Script: `a--`, Input: map[string]interface{}{"a": true}, RunOutput: int64(0), Output: map[string]interface{}{"a": int64(0)}},
		{Script: `a--`, Input: map[string]interface{}{"a": int32(2)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a--`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a--`, Input: map[string]interface{}{"a": float32(2.5)}, RunOutput: 1.5, Output: map[string]interface{}{"a": 1.5}},
		{Script: `a--`, Input: map[string]interface{}{"a": 2.5}, RunOutput: 1.5, Output: map[string]interface{}{"a": 1.5}},
		{Script: `a--`, Input: map[string]interface{}{"a": "2"}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a--`, Input: map[string]interface{}{"a": "a"}, RunOutput: int64(-1), Output: map[string]interface{}{"a": int64(-1)}},

		{Script: `1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1--`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `z++`, RunError: fmt.Errorf("undefined symbol \"z\""), RunOutput: nil},
		{Script: `z--`, RunError: fmt.Errorf("undefined symbol \"z\""), RunOutput: nil},
		{Script: `!(1++)`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1 + 1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1 - 1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1 * 1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1 / 1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1++ + 1`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1++ - 1`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1++ * 1`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1++ / 1`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},

		{Script: `a = 1; b = 2; a + b`, RunOutput: int64(3)},
		{Script: `a = [1]; b = 2; a[0] + b`, RunOutput: int64(3)},
		{Script: `a = 1; b = [2]; a + b[0]`, RunOutput: int64(3)},
		{Script: `a = 2; b = 1; a - b`, RunOutput: int64(1)},
		{Script: `a = [2]; b = 1; a[0] - b`, RunOutput: int64(1)},
		{Script: `a = 2; b = [1]; a - b[0]`, RunOutput: int64(1)},
		{Script: `a = 1; b = 2; a * b`, RunOutput: int64(2)},
		{Script: `a = [1]; b = 2; a[0] * b`, RunOutput: int64(2)},
		{Script: `a = 1; b = [2]; a * b[0]`, RunOutput: int64(2)},
		{Script: `a = 4; b = 2; a / b`, RunOutput: float64(2)},
		{Script: `a = [4]; b = 2; a[0] / b`, RunOutput: float64(2)},
		{Script: `a = 4; b = [2]; a / b[0]`, RunOutput: float64(2)},

		{Script: `a += 1`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(3), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a -= 1`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a *= 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(4), Output: map[string]interface{}{"a": int64(4)}},
		{Script: `a /= 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: float64(1), Output: map[string]interface{}{"a": float64(1)}},
		{Script: `a += 1`, Input: map[string]interface{}{"a": 2.1}, RunOutput: 3.1, Output: map[string]interface{}{"a": 3.1}},
		{Script: `a -= 1`, Input: map[string]interface{}{"a": 2.1}, RunOutput: 1.1, Output: map[string]interface{}{"a": 1.1}},
		{Script: `a *= 2`, Input: map[string]interface{}{"a": 2.1}, RunOutput: 4.2, Output: map[string]interface{}{"a": 4.2}},
		{Script: `a /= 2`, Input: map[string]interface{}{"a": 6.5}, RunOutput: 3.25, Output: map[string]interface{}{"a": 3.25}},

		{Script: `a &= 1`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(0), Output: map[string]interface{}{"a": int64(0)}},
		{Script: `a &= 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a &= 1`, Input: map[string]interface{}{"a": 2.1}, RunOutput: int64(0), Output: map[string]interface{}{"a": int64(0)}},
		{Script: `a &= 2`, Input: map[string]interface{}{"a": 2.1}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},

		{Script: `a |= 1`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(3), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a |= 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a |= 1`, Input: map[string]interface{}{"a": 2.1}, RunOutput: int64(3), Output: map[string]interface{}{"a": int64(3)}},
		{Script: `a |= 2`, Input: map[string]interface{}{"a": 2.1}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},

		{Script: `a << 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(8), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a >> 2`, Input: map[string]interface{}{"a": int64(8)}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(8)}},
		{Script: `a << 2`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: int64(8), Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a >> 2`, Input: map[string]interface{}{"a": float64(8)}, RunOutput: int64(2), Output: map[string]interface{}{"a": float64(8)}},

		{Script: `a % 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(0), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a % 3`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a % 2`, Input: map[string]interface{}{"a": 2.1}, RunOutput: int64(0), Output: map[string]interface{}{"a": 2.1}},
		{Script: `a % 3`, Input: map[string]interface{}{"a": 2.1}, RunOutput: int64(2), Output: map[string]interface{}{"a": 2.1}},

		{Script: `a * 4`, Input: map[string]interface{}{"a": "a"}, RunOutput: "aaaa", Output: map[string]interface{}{"a": "a"}},
		{Script: `a * 4.0`, Input: map[string]interface{}{"a": "a"}, RunOutput: float64(0), Output: map[string]interface{}{"a": "a"}},

		{Script: `-a`, Input: map[string]interface{}{"a": nil}, RunOutput: float64(-0), Output: map[string]interface{}{"a": nil}},
		{Script: `-a`, Input: map[string]interface{}{"a": int32(1)}, RunOutput: int64(-1), Output: map[string]interface{}{"a": int32(1)}},
		{Script: `-a`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(-2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `-a`, Input: map[string]interface{}{"a": float32(3.5)}, RunOutput: -3.5, Output: map[string]interface{}{"a": float32(3.5)}},
		{Script: `-a`, Input: map[string]interface{}{"a": 4.5}, RunOutput: -4.5, Output: map[string]interface{}{"a": 4.5}},
		{Script: `-a`, Input: map[string]interface{}{"a": "a"}, RunOutput: float64(-0), Output: map[string]interface{}{"a": "a"}},
		{Script: `-a`, Input: map[string]interface{}{"a": "1"}, RunOutput: float64(-1), Output: map[string]interface{}{"a": "1"}},

		{Script: `^a`, Input: map[string]interface{}{"a": nil}, RunOutput: int64(-1), Output: map[string]interface{}{"a": nil}},
		{Script: `^a`, Input: map[string]interface{}{"a": "a"}, RunOutput: int64(-1), Output: map[string]interface{}{"a": "a"}},
		{Script: `^a`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: int64(-3), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `^a`, Input: map[string]interface{}{"a": 2.1}, RunOutput: int64(-3), Output: map[string]interface{}{"a": 2.1}},

		{Script: `!true`, RunOutput: false},
		{Script: `!false`, RunOutput: true},
		{Script: `!1`, RunOutput: false},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestComparisonOperators(t *testing.T) {
	tests := []*Test{
		{Script: `1++ == 2`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `2 == 1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1++ || true`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `false || 1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `1++ && true`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},
		{Script: `true && 1++`, RunError: fmt.Errorf("invalid operation"), RunOutput: nil},

		{Script: `a == 1`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a == 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a != 1`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a != 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a == 1.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a == 2.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a != 1.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a != 2.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},

		{Script: `a == 1`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a == 2`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a != 1`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a != 2`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a == 1.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a == 2.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a != 1.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a != 2.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},

		{Script: `a == nil`, Input: map[string]interface{}{"a": nil}, RunOutput: true, Output: map[string]interface{}{"a": nil}},
		{Script: `a == nil`, Input: map[string]interface{}{"a": nil}, RunOutput: true, Output: map[string]interface{}{"a": nil}},
		{Script: `a == nil`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a == nil`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a == 2`, Input: map[string]interface{}{"a": nil}, RunOutput: false, Output: map[string]interface{}{"a": nil}},
		{Script: `a == 2.0`, Input: map[string]interface{}{"a": nil}, RunOutput: false, Output: map[string]interface{}{"a": nil}},

		{Script: `1 == 1.0`, RunOutput: true},
		{Script: `1 != 1.0`, RunOutput: false},
		{Script: `"a" != "a"`, RunOutput: false},

		{Script: `a > 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a > 1`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a < 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a < 3`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a > 2.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a > 1.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a < 2.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a < 3.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},

		{Script: `a > 2`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a > 1`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a < 2`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a < 3`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a > 2.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a > 1.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a < 2.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a < 3.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},

		{Script: `a >= 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a >= 3`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a <= 2`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a <= 3`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a >= 2.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a >= 3.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a <= 2.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a <= 3.0`, Input: map[string]interface{}{"a": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2)}},

		{Script: `a >= 2`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a >= 3`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a <= 2`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a <= 3`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a >= 2.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a >= 3.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: false, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a <= 2.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},
		{Script: `a <= 3.0`, Input: map[string]interface{}{"a": float64(2)}, RunOutput: true, Output: map[string]interface{}{"a": float64(2)}},

		{Script: `a = false; b = false; a && b`, RunOutput: false},
		{Script: `a = false; b = true; a && b`, RunOutput: false},
		{Script: `a = true; b = false; a && b`, RunOutput: false},
		{Script: `a = true; b = true; a && b`, RunOutput: true},
		{Script: `a = false; b = false; a || b`, RunOutput: false},
		{Script: `a = false; b = true; a || b`, RunOutput: true},
		{Script: `a = true; b = false; a || b`, RunOutput: true},
		{Script: `a = true; b = true; a || b`, RunOutput: true},

		{Script: `a = [false]; b = false; a[0] && b`, RunOutput: false},
		{Script: `a = [false]; b = true; a[0] && b`, RunOutput: false},
		{Script: `a = [true]; b = false; a[0] && b`, RunOutput: false},
		{Script: `a = [true]; b = true; a[0] && b`, RunOutput: true},
		{Script: `a = [false]; b = false; a[0] || b`, RunOutput: false},
		{Script: `a = [false]; b = true; a[0] || b`, RunOutput: true},
		{Script: `a = [true]; b = false; a[0] || b`, RunOutput: true},
		{Script: `a = [true]; b = true; a[0] || b`, RunOutput: true},

		{Script: `a = false; b = [false]; a && b[0]`, RunOutput: false},
		{Script: `a = false; b = [true]; a && b[0]`, RunOutput: false},
		{Script: `a = true; b = [false]; a && b[0]`, RunOutput: false},
		{Script: `a = true; b = [true]; a && b[0]`, RunOutput: true},
		{Script: `a = false; b = [false]; a || b[0]`, RunOutput: false},
		{Script: `a = false; b = [true]; a || b[0]`, RunOutput: true},
		{Script: `a = true; b = [false]; a || b[0]`, RunOutput: true},
		{Script: `a = true; b = [true]; a || b[0]`, RunOutput: true},

		{Script: `0 && 0`, RunOutput: false},
		{Script: `0 && 1`, RunOutput: false},
		{Script: `1 && 0`, RunOutput: false},
		{Script: `1 && 1`, RunOutput: true},
		{Script: `0 || 0`, RunOutput: false},
		{Script: `0 || 1`, RunOutput: true},
		{Script: `1 || 0`, RunOutput: true},
		{Script: `1 || 1`, RunOutput: true},

		{Script: `1 == 1 && 1 == 1`, RunOutput: true},
		{Script: `1 == 1 && 1 == 2`, RunOutput: false},
		{Script: `1 == 2 && 1 == 1`, RunOutput: false},
		{Script: `1 == 2 && 1 == 2`, RunOutput: false},
		{Script: `1 == 1 || 1 == 1`, RunOutput: true},
		{Script: `1 == 1 || 1 == 2`, RunOutput: true},
		{Script: `1 == 2 || 1 == 1`, RunOutput: true},
		{Script: `1 == 2 || 1 == 2`, RunOutput: false},

		{Script: `true == "1"`, RunOutput: true},
		{Script: `true == "t"`, RunOutput: true},
		{Script: `true == "T"`, RunOutput: true},
		{Script: `true == "true"`, RunOutput: true},
		{Script: `true == "TRUE"`, RunOutput: true},
		{Script: `true == "True"`, RunOutput: true},
		{Script: `true == "false"`, RunOutput: false},
		{Script: `false == "0"`, RunOutput: true},
		{Script: `false == "f"`, RunOutput: true},
		{Script: `false == "F"`, RunOutput: true},
		{Script: `false == "false"`, RunOutput: true},
		{Script: `false == "false"`, RunOutput: true},
		{Script: `false == "FALSE"`, RunOutput: true},
		{Script: `false == "False"`, RunOutput: true},
		{Script: `false == "true"`, RunOutput: false},
		{Script: `false == "foo"`, RunOutput: false},
		{Script: `true == "foo"`, RunOutput: true},

		{Script: `0 == "0"`, RunOutput: true},
		{Script: `"1.0" == 1`, RunOutput: true},
		{Script: `1 == "1"`, RunOutput: true},
		{Script: `0.0 == "0"`, RunOutput: true},
		{Script: `0.0 == "0.0"`, RunOutput: true},
		{Script: `1.0 == "1.0"`, RunOutput: true},
		{Script: `1.2 == "1.2"`, RunOutput: true},
		{Script: `"7" == 7.2`, RunOutput: false},
		{Script: `1.2 == "1"`, RunOutput: false},
		{Script: `"1.1" == 1`, RunOutput: false},
		{Script: `0 == "1"`, RunOutput: false},

		{Script: `a == b`, Input: map[string]interface{}{"a": reflect.Value{}, "b": reflect.Value{}}, RunOutput: true, Output: map[string]interface{}{"a": reflect.Value{}, "b": reflect.Value{}}},
		{Script: `a == b`, Input: map[string]interface{}{"a": reflect.Value{}, "b": true}, RunOutput: false, Output: map[string]interface{}{"a": reflect.Value{}, "b": true}},
		{Script: `a == b`, Input: map[string]interface{}{"a": true, "b": reflect.Value{}}, RunOutput: false, Output: map[string]interface{}{"a": true, "b": reflect.Value{}}},

		{Script: `a == b`, Input: map[string]interface{}{"a": nil, "b": nil}, RunOutput: true, Output: map[string]interface{}{"a": nil, "b": nil}},
		{Script: `a == b`, Input: map[string]interface{}{"a": nil, "b": true}, RunOutput: false, Output: map[string]interface{}{"a": nil, "b": true}},
		{Script: `a == b`, Input: map[string]interface{}{"a": true, "b": nil}, RunOutput: false, Output: map[string]interface{}{"a": true, "b": nil}},

		{Script: `a == b`, Input: map[string]interface{}{"a": false, "b": false}, RunOutput: true, Output: map[string]interface{}{"a": false, "b": false}},
		{Script: `a == b`, Input: map[string]interface{}{"a": false, "b": true}, RunOutput: false, Output: map[string]interface{}{"a": false, "b": true}},
		{Script: `a == b`, Input: map[string]interface{}{"a": true, "b": false}, RunOutput: false, Output: map[string]interface{}{"a": true, "b": false}},
		{Script: `a == b`, Input: map[string]interface{}{"a": true, "b": true}, RunOutput: true, Output: map[string]interface{}{"a": true, "b": true}},

		{Script: `a == b`, Input: map[string]interface{}{"a": int32(1), "b": int32(1)}, RunOutput: true, Output: map[string]interface{}{"a": int32(1), "b": int32(1)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": int32(1), "b": int32(2)}, RunOutput: false, Output: map[string]interface{}{"a": int32(1), "b": int32(2)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": int32(2), "b": int32(1)}, RunOutput: false, Output: map[string]interface{}{"a": int32(2), "b": int32(1)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": int32(2), "b": int32(2)}, RunOutput: true, Output: map[string]interface{}{"a": int32(2), "b": int32(2)}},

		{Script: `a == b`, Input: map[string]interface{}{"a": int64(1), "b": int64(1)}, RunOutput: true, Output: map[string]interface{}{"a": int64(1), "b": int64(1)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": int64(1), "b": int64(2)}, RunOutput: false, Output: map[string]interface{}{"a": int64(1), "b": int64(2)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": int64(2), "b": int64(1)}, RunOutput: false, Output: map[string]interface{}{"a": int64(2), "b": int64(1)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": int64(2), "b": int64(2)}, RunOutput: true, Output: map[string]interface{}{"a": int64(2), "b": int64(2)}},

		{Script: `a == b`, Input: map[string]interface{}{"a": float32(1.1), "b": float32(1.1)}, RunOutput: true, Output: map[string]interface{}{"a": float32(1.1), "b": float32(1.1)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": float32(1.1), "b": float32(2.2)}, RunOutput: false, Output: map[string]interface{}{"a": float32(1.1), "b": float32(2.2)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": float32(2.2), "b": float32(1.1)}, RunOutput: false, Output: map[string]interface{}{"a": float32(2.2), "b": float32(1.1)}},
		{Script: `a == b`, Input: map[string]interface{}{"a": float32(2.2), "b": float32(2.2)}, RunOutput: true, Output: map[string]interface{}{"a": float32(2.2), "b": float32(2.2)}},

		{Script: `a == b`, Input: map[string]interface{}{"a": 1.1, "b": 1.1}, RunOutput: true, Output: map[string]interface{}{"a": 1.1, "b": 1.1}},
		{Script: `a == b`, Input: map[string]interface{}{"a": 1.1, "b": 2.2}, RunOutput: false, Output: map[string]interface{}{"a": 1.1, "b": 2.2}},
		{Script: `a == b`, Input: map[string]interface{}{"a": 2.2, "b": 1.1}, RunOutput: false, Output: map[string]interface{}{"a": 2.2, "b": 1.1}},
		{Script: `a == b`, Input: map[string]interface{}{"a": 2.2, "b": 2.2}, RunOutput: true, Output: map[string]interface{}{"a": 2.2, "b": 2.2}},

		{Script: `a == b`, Input: map[string]interface{}{"a": 'a', "b": 'a'}, RunOutput: true, Output: map[string]interface{}{"a": 'a', "b": 'a'}},
		{Script: `a == b`, Input: map[string]interface{}{"a": 'a', "b": 'b'}, RunOutput: false, Output: map[string]interface{}{"a": 'a', "b": 'b'}},
		{Script: `a == b`, Input: map[string]interface{}{"a": 'b', "b": 'a'}, RunOutput: false, Output: map[string]interface{}{"a": 'b', "b": 'a'}},
		{Script: `a == b`, Input: map[string]interface{}{"a": 'b', "b": 'b'}, RunOutput: true, Output: map[string]interface{}{"a": 'b', "b": 'b'}},

		{Script: `a == b`, Input: map[string]interface{}{"a": "a", "b": "a"}, RunOutput: true, Output: map[string]interface{}{"a": "a", "b": "a"}},
		{Script: `a == b`, Input: map[string]interface{}{"a": "a", "b": "b"}, RunOutput: false, Output: map[string]interface{}{"a": "a", "b": "b"}},
		{Script: `a == b`, Input: map[string]interface{}{"a": "b", "b": "a"}, RunOutput: false, Output: map[string]interface{}{"a": "b", "b": "a"}},
		{Script: `a == b`, Input: map[string]interface{}{"a": "b", "b": "b"}, RunOutput: true, Output: map[string]interface{}{"a": "b", "b": "b"}},

		{Script: `b = "\"a\""; a == b`, Input: map[string]interface{}{"a": `"a"`}, RunOutput: true, Output: map[string]interface{}{"a": `"a"`, "b": `"a"`}},

		{Script: `a = "test"; a == "test"`, RunOutput: true},
		{Script: `a = "test"; a[0:1] == "t"`, RunOutput: true},
		{Script: `a = "test"; a[0:2] == "te"`, RunOutput: true},
		{Script: `a = "test"; a[1:3] == "es"`, RunOutput: true},
		{Script: `a = "test"; a[0:4] == "test"`, RunOutput: true},

		{Script: `a = "a b"; a[1] == ' '`, RunOutput: true},
		{Script: `a = "test"; a[0] == 't'`, RunOutput: true},
		{Script: `a = "test"; a[1] == 'e'`, RunOutput: true},
		{Script: `a = "test"; a[3] == 't'`, RunOutput: true},

		{Script: `a = "a b"; a[1] != ' '`, RunOutput: false},
		{Script: `a = "test"; a[0] != 't'`, RunOutput: false},
		{Script: `a = "test"; a[1] != 'e'`, RunOutput: false},
		{Script: `a = "test"; a[3] != 't'`, RunOutput: false},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestTernaryOperator(t *testing.T) {
	tests := []*Test{
		{Script: `a = a ? 1 : 2`, RunError: fmt.Errorf("undefined symbol \"a\"")},
		{Script: `a = z ? 1 : 2`, RunError: fmt.Errorf("undefined symbol \"z\"")},
		{Script: `a = 0; a = a ? 1 : z`, RunError: fmt.Errorf("undefined symbol \"z\"")},
		{Script: `a = 1; a = a ? z : 1`, RunError: fmt.Errorf("undefined symbol \"z\"")},
		{Script: `a = b[1] ? 2 : 1`, Input: map[string]interface{}{"b": []interface{}{}}, RunError: fmt.Errorf("index out of range")},
		{Script: `a = b[1][2] ? 2 : 1`, Input: map[string]interface{}{"b": []interface{}{}}, RunError: fmt.Errorf("index out of range")},
		{Script: `a = b["test"][1] ? 2 : 1`, Input: map[string]interface{}{"b": map[string]interface{}{"test": 2}}, RunError: fmt.Errorf("type int does not support index operation")},

		{Script: `a = 1 ? 2 : z`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = -1 ? 2 : 1`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = true ? 2 : 1`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = false ? 2 : 1`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = "true" ? 2 : 1`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = "false" ? 2 : 1`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = "-1" ? 2 : 1`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = "0" ? 2 : 1`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = "0.0" ? 2 : 1`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = "2" ? 2 : 1`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = b ? 2 : 1`, Input: map[string]interface{}{"b": int64(0)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = b ? 2 : 1`, Input: map[string]interface{}{"b": int64(2)}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = b ? 2 : 1`, Input: map[string]interface{}{"b": 0.0}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = b ? 2 : 1`, Input: map[string]interface{}{"b": 2.0}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = b ? 2 : 1.0`, Input: map[string]interface{}{"b": 0.0}, RunOutput: 1.0, Output: map[string]interface{}{"a": 1.0}},
		{Script: `a = b ? 2 : 1.0`, Input: map[string]interface{}{"b": 0.1}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = b ? 2 : 1.0`, Input: map[string]interface{}{"b": nil}, RunOutput: 1.0, Output: map[string]interface{}{"a": 1.0}},
		{Script: `a = nil ? 2 : 1`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = b ? 2 : 1`, Input: map[string]interface{}{"b": []interface{}{}}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = b ? 2 : 1`, Input: map[string]interface{}{"b": map[string]interface{}{}}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = [] ? 2 : 1`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a = [2] ? 2 : 1`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = b ? 2 : 1`, Input: map[string]interface{}{"b": map[string]interface{}{"test": int64(2)}}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `a = b["test"] ? 2 : 1`, Input: map[string]interface{}{"b": map[string]interface{}{"test": int64(2)}}, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `b = "test"; a = b ? 2 : "empty"`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `b = "test"; a = b[1:3] ? 2 : "empty"`, RunOutput: int64(2), Output: map[string]interface{}{"a": int64(2)}},
		{Script: `b = "test"; a = b[2:2] ? 2 : "empty"`, RunOutput: "empty", Output: map[string]interface{}{"a": "empty"}},
		{Script: `b = "0.0"; a = false ? 2 : b ? 3 : 1`, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `b = "true"; a = false ? 2 : b ? 3 : 1`, RunOutput: int64(3), Output: map[string]interface{}{"a": int64(3)}},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestNilCoalescingOperator(t *testing.T) {
	tests := []*Test{
		{Script: `nil ?? nil`, RunOutput: nil},
		{Script: `false ?? nil`, RunOutput: false},
		{Script: `true ?? nil`, RunOutput: true},
		{Script: `nil ?? false`, RunOutput: false},
		{Script: `nil ?? true`, RunOutput: true},
		{Script: `1 ?? nil`, RunOutput: int64(1)},
		{Script: `1 ?? 2`, RunOutput: int64(1)},
		{Script: `nil ?? 1`, RunOutput: int64(1)},

		{Script: `a ?? 1`, RunOutput: int64(1)},
		{Script: `a ?? b`, RunError: fmt.Errorf("undefined symbol \"b\"")},

		{Script: `a ?? 2`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a ?? b`, Input: map[string]interface{}{"a": int64(1)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1)}},
		{Script: `a ?? b`, Input: map[string]interface{}{"a": int64(1), "b": int64(2)}, RunOutput: int64(1), Output: map[string]interface{}{"a": int64(1), "b": int64(2)}},
		{Script: `a ?? b`, Input: map[string]interface{}{"a": nil, "b": int64(2)}, RunOutput: int64(2), Output: map[string]interface{}{"a": nil, "b": int64(2)}},

		{Script: `[] ?? 1`, RunOutput: []interface{}{}},
		{Script: `{} ?? 1`, RunOutput: map[interface{}]interface{}{}},

		// test nil array and map
		{Script: `a ?? 5`, Input: map[string]interface{}{"a": testSliceEmpty}, RunOutput: int64(5), Output: map[string]interface{}{"a": testSliceEmpty}},
		{Script: `a ?? 6`, Input: map[string]interface{}{"a": testMapEmpty}, RunOutput: int64(6), Output: map[string]interface{}{"a": testMapEmpty}},
	}
	runTests(t, tests, &Options{Debug: true})
}

func TestOperatorPrecedence(t *testing.T) {
	tests := []*Test{
		// test && > ||
		{Script: `true || true && false`, RunOutput: true},
		{Script: `(true || true) && false`, RunOutput: false},
		{Script: `false && true || true`, RunOutput: true},
		{Script: `false && (true || true)`, RunOutput: false},

		// test == > ||
		{Script: `0 == 1 || 1 == 1`, RunOutput: true},
		{Script: `0 == (1 || 1) == 1`, RunOutput: false},

		// test + > ==
		{Script: `1 + 2 == 2 + 1`, RunOutput: true},
		{Script: `1 + (2 == 2) + 1`, RunOutput: int64(3)},

		// test * > +
		{Script: `2 * 3 + 4 * 5`, RunOutput: int64(26)},
		{Script: `2 * (3 + 4) * 5`, RunOutput: int64(70)},

		// test * > &&
		{Script: `2 * 0 && 3 * 4`, RunOutput: false},
		{Script: `2 * (0 && 3) * 4`, RunOutput: int64(0)},

		// test ++ > *
		{Script: `a = 1; b = 2; a++ * b++`, RunOutput: int64(6)},

		// test ++ > *
		{Script: `a = 1; b = 2; a++ * b++`, RunOutput: int64(6)},

		// test unary - > +
		{Script: `a = 1; b = 2; -a + b`, RunOutput: int64(1)},
		{Script: `a = 1; b = 2; -(a + b)`, RunOutput: int64(-3)},
		{Script: `a = 1; b = 2; a + -b`, RunOutput: int64(-1)},

		// test ! > ||
		{Script: `!true || true`, RunOutput: true},
		{Script: `!(true || true)`, RunOutput: false},
	}
	runTests(t, tests, &Options{Debug: true})
}

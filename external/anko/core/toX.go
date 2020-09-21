package core

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"project/external/anko/env"
)

// ImportToX adds all the toX to the env given.
func ImportToX(e *env.Env) {
	_ = e.Define("toBool", toBool)
	_ = e.Define("toString", toString)
	_ = e.Define("toInt", toInt)
	_ = e.Define("toFloat", toFloat)
	_ = e.Define("toChar", toChar)
	_ = e.Define("toRune", toRune)
	_ = e.Define("toBoolSlice", toBoolSlice)
	_ = e.Define("toStringSlice", toStringSlice)
	_ = e.Define("toIntSlice", toIntSlice)
	_ = e.Define("toFloatSlice", toFloatSlice)
	_ = e.Define("toByteSlice", toByteSlice)
	_ = e.Define("toRuneSlice", toRuneSlice)
	_ = e.Define("toDuration", toDuration)
}

func toBool(v interface{}) bool {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return false
	}
	nt := reflect.TypeOf(true)
	if rv.Type().ConvertibleTo(nt) {
		return rv.Convert(nt).Bool()
	}
	if rv.Type().ConvertibleTo(reflect.TypeOf(1.0)) &&
		rv.Convert(reflect.TypeOf(1.0)).Float() > 0.0 {
		return true
	}
	if rv.Kind() == reflect.String {
		s := strings.ToLower(v.(string))
		if s == "y" || s == "yes" {
			return true
		}
		b, err := strconv.ParseBool(s)
		if err == nil {
			return b
		}
	}
	return false
}

func toString(v interface{}) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(v)
}

func toInt(v interface{}) int64 {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return 0
	}
	nt := reflect.TypeOf(1)
	if rv.Type().ConvertibleTo(nt) {
		return rv.Convert(nt).Int()
	}
	if rv.Kind() == reflect.String {
		i, err := strconv.ParseInt(v.(string), 10, 64)
		if err == nil {
			return i
		}
		f, err := strconv.ParseFloat(v.(string), 64)
		if err == nil {
			return int64(f)
		}
	}
	if rv.Kind() == reflect.Bool {
		if v.(bool) {
			return 1
		}
	}
	return 0
}

func toFloat(v interface{}) float64 {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return 0
	}
	nt := reflect.TypeOf(1.0)
	if rv.Type().ConvertibleTo(nt) {
		return rv.Convert(nt).Float()
	}
	if rv.Kind() == reflect.String {
		f, err := strconv.ParseFloat(v.(string), 64)
		if err == nil {
			return f
		}
	}
	if rv.Kind() == reflect.Bool {
		if v.(bool) {
			return 1.0
		}
	}
	return 0.0
}

func toChar(s rune) string {
	return string(s)
}

func toRune(s string) rune {
	if len(s) == 0 {
		return 0
	}
	return []rune(s)[0]
}

func toBoolSlice(v []interface{}) []bool {
	var result []bool
	toSlice(v, &result)
	return result
}

func toStringSlice(v []interface{}) []string {
	var result []string
	toSlice(v, &result)
	return result
}

func toIntSlice(v []interface{}) []int64 {
	var result []int64
	toSlice(v, &result)
	return result
}

func toFloatSlice(v []interface{}) []float64 {
	var result []float64
	toSlice(v, &result)
	return result
}

func toByteSlice(s string) []byte {
	return []byte(s)
}

func toRuneSlice(s string) []rune {
	return []rune(s)
}

func toDuration(v int64) time.Duration {
	return time.Duration(v)
}

// toSlice takes in a "generic" slice and converts and copies
// it's elements into the typed slice pointed at by ptr.
// Note that this is a costly operation.
func toSlice(from []interface{}, ptr interface{}) {
	// Value of the pointer to the target
	obj := reflect.Indirect(reflect.ValueOf(ptr))
	// We can't just convert from interface{} to whatever the
	// target is (diff memory layout), so we need to create a
	// New slice of the proper type and copy the values individually
	t := reflect.TypeOf(ptr).Elem()
	tt := t.Elem()
	slice := reflect.MakeSlice(t, len(from), len(from))
	// Copying the data, val is an addressable Pointer of the actual target type
	val := reflect.Indirect(reflect.New(tt))
	for i := 0; i < len(from); i++ {
		v := reflect.ValueOf(from[i])
		if v.IsValid() && v.Type().ConvertibleTo(tt) {
			val.Set(v.Convert(tt))
		} else {
			val.Set(reflect.Zero(tt))
		}
		slice.Index(i).Set(val)
	}
	// Ok now assign our slice to the target pointer
	obj.Set(slice)
}

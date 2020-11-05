package testsuite

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/xpanic"
)

// ContainZeroValue is used to check decoder.Unmarshal is apply value to each field,
// it will check each field value is zero in structure for prevent programmer forget
// add new field to toml(and other) options file if add new field to Option structure.
func ContainZeroValue(t *testing.T, v interface{}) {
	str := containZeroValue("", v)
	require.True(t, str == "", str)
}

func containZeroValue(father string, v interface{}) (result string) {
	ok, result := checkSpecialType(father, v)
	if ok {
		return
	}
	typ := reflect.TypeOf(v)
	var value reflect.Value
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "containZeroValue")
			result = fmt.Sprint(father+typ.Name(), " with panic occurred")
		}
	}()
	if typ.Kind() == reflect.Ptr {
		value = reflect.ValueOf(v)
		typ = value.Type()
		if value.IsNil() { // check is nil point
			return father + typ.Name() + " is nil point"
		}
		value = value.Elem()
		typ = value.Type()
	} else {
		value = reflect.ValueOf(v)
	}
	return walkOptions(father, typ, value)
}

func checkSpecialType(father string, v interface{}) (bool, string) {
	var typ string
	switch val := v.(type) {
	case *time.Time:
		if val != nil && !val.IsZero() {
			return true, ""
		}
		typ = "time.Time"
	case time.Time:
		if !val.IsZero() {
			return true, ""
		}
		typ = "time.Time"
	default:
		return false, ""
	}
	if father == "" {
		return true, typ + " is zero value"
	}
	return true, father + " is zero value"
}

func walkOptions(father string, typ reflect.Type, value reflect.Value) string {
	for i := 0; i < value.NumField(); i++ {
		fieldType := typ.Field(i)
		fieldValue := value.Field(i)
		// skip unexported field
		if fieldType.PkgPath != "" && !fieldType.Anonymous {
			continue
		}
		// skip filed with check tag
		fieldTag, ok := fieldType.Tag.Lookup("testsuite")
		if ok {
			if fieldTag == "" {
				const format = "empty value in testsuite tag, field: \"%s\""
				panic(fmt.Sprintf(format, fieldType.Name))
			}
			if fieldTag == "-" {
				continue
			}
		}
		switch fieldType.Type.Kind() {
		case reflect.Struct, reflect.Ptr:
			var f string
			if father == "" {
				f = typ.Name() + "." + fieldType.Name
			} else {
				f = father + "." + fieldType.Name
			}
			result := containZeroValue(f, fieldValue.Interface())
			if result != "" {
				return result
			}
		case reflect.Chan, reflect.Func, reflect.Interface,
			reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
			continue
		default:
			if !fieldValue.IsZero() {
				continue
			}
			const format = "%s.%s is zero value"
			if father == "" {
				return fmt.Sprintf(format, typ.Name(), fieldType.Name)
			}
			return fmt.Sprintf(format, father, fieldType.Name)
		}
	}
	return ""
}

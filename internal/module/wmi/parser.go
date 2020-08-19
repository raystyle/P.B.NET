// +build windows

package wmi

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/pkg/errors"

	"project/internal/xpanic"
)

// parseExecQueryResult is used to parse ExecQuery result to destination interface.
func parseExecQueryResult(objects []*Object, dst interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "parseExecQueryResult")
		}
	}()
	// walk destination structure for get structure fields
	val := reflect.ValueOf(dst)
	slice, elem := checkDstType(dst, val)
	// make slice
	objectsLen := len(objects)
	val = val.Elem() // slice
	val.Set(reflect.MakeSlice(slice, objectsLen, objectsLen))
	// check slice element is pointer
	elemIsPtr := elem.Kind() == reflect.Ptr
	if elemIsPtr {
		elem = elem.Elem()
	}
	fields := getStructFields(elem)
	for i, object := range objects {
		element := val.Index(i)
		if elemIsPtr {
			element.Set(reflect.New(elem))
			element = element.Elem()
		}
		for j := 0; j < len(fields); j++ {
			field := fields[j]

			if field == "" {
				continue
			}

			prop, err := object.GetProperty(field)
			if err != nil {
				return errors.Wrapf(err, "failed to get property \"%s\"", field)
			}

			fmt.Println(prop.Value(), reflect.TypeOf(prop.Value()))
			// fmt.Println()

			if prop.raw.VT == ole.VT_NULL {
				continue
			}

			setValue(element.Field(j), prop)

			prop.Clear()
		}
	}
	return
}

func checkDstType(dst interface{}, val reflect.Value) (slice, elem reflect.Type) {
	typ := reflect.TypeOf(dst)
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("destination interface is not slice pointer or is nil")
	}
	slice = typ.Elem()
	if slice.Kind() != reflect.Slice {
		panic("destination pointer is not point to slice")
	}
	elem = slice.Elem()
	switch elem.Kind() {
	case reflect.Struct:
	case reflect.Ptr:
		if elem.Elem().Kind() != reflect.Struct {
			panic("destination slice element pointer is not point to structure")
		}
	default:
		panic("destination slice element is not structure or pointer")
	}
	return
}

func getStructFields(elem reflect.Type) []string {
	l := elem.NumField()
	fields := make([]string, l)
	for i := 0; i < l; i++ {
		field := elem.Field(i)
		// skip unexported field
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		// check structure tag
		fieldTag, ok := field.Tag.Lookup("wmi")
		if !ok {
			fields[i] = field.Name
			continue
		}
		switch fieldTag {
		case "-":
		case "":
			panic("empty value in wmi tag")
		default:
			fields[i] = fieldTag
		}
	}
	return fields
}

// ErrFieldMismatch is returned when a field is to be loaded into a different
// type than the one it was stored from, or when a field is missing or
// unexported in the destination struct.
// StructType is the type of the struct pointed to by the destination argument.
type ErrFieldMismatch struct {
	FieldName  string
	StructType reflect.Type
	Reason     string
}

func (e *ErrFieldMismatch) Error() string {
	return fmt.Sprintf("wmi: cannot load field %q into a %q: %s",
		e.FieldName, e.StructType, e.Reason)
}

var timeType = reflect.TypeOf(time.Time{})

func setValue(field reflect.Value, prop *Object) error {
	fieldType := field.Type()
	name := fieldType.Name()
	if !field.CanSet() {
		return &ErrFieldMismatch{
			FieldName:  name,
			StructType: fieldType,
			Reason:     "can not set value",
		}
	}
	switch val := prop.Value().(type) {
	case int, int8, int16, int32, int64:
		v := reflect.ValueOf(val).Int()
		switch field.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(v)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(uint64(v))
		default:
			return &ErrFieldMismatch{
				FieldName:  name,
				StructType: fieldType,
				Reason:     "not an integer type",
			}
		}
	case uint, uint8, uint16, uint32, uint64:
		v := reflect.ValueOf(val).Uint()
		switch field.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(int64(v))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(v)
		default:
			return &ErrFieldMismatch{
				FieldName:  name,
				StructType: fieldType,
				Reason:     "not an unsigned integer type",
			}
		}
	case float32, float64:
		v := reflect.ValueOf(val).Float()
		switch field.Kind() {
		case reflect.Float32, reflect.Float64:
			field.SetFloat(v)
		default:
			return &ErrFieldMismatch{
				FieldName:  name,
				StructType: fieldType,
				Reason:     "not a float type",
			}
		}
	case bool:
		switch field.Kind() {
		case reflect.Bool:
			field.SetBool(val)
		default:
			return &ErrFieldMismatch{
				FieldName:  name,
				StructType: fieldType,
				Reason:     "not a bool type",
			}
		}
	case string:

	default:

	}
	return nil
}

// BuildWQL is used to build structure to WQL string.
//
// type testWin32Process struct {
//     Name   string
//     PID    uint32 `wmi:"ProcessId"`
//     Ignore string `wmi:"-"`
// }
//
// to select Name, ProcessId from Win32_Process
func BuildWQL(structure interface{}, form string) string {
	fields := getStructFields(reflect.TypeOf(structure))
	fieldsLen := len(fields)
	// remove empty string
	f := make([]string, 0, fieldsLen)
	for i := 0; i < fieldsLen; i++ {
		if fields[i] != "" {
			f = append(f, fields[i])
		}
	}
	return "select " + strings.Join(f, ", ") + " from " + form
}

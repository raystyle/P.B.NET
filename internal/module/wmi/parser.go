// +build windows

package wmi

import (
	"fmt"
	"reflect"
	"strconv"
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
	slice, elem := checkExecQueryDstType(dst, val)
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
			// skipped field
			if field == "" {
				continue
			}
			err = setProperty(element.Field(j), object, field)
			if err != nil {
				return err
			}
		}
	}
	return
}

func checkExecQueryDstType(dst interface{}, val reflect.Value) (slice, elem reflect.Type) {
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

// parseExecMethodResult is used to parse ExecMethod result to destination interface.
func parseExecMethodResult(objects []*Object, dst interface{}) (err error) {
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

func setProperty(field reflect.Value, object *Object, name string) error {
	prop, err := object.GetProperty(name)
	if err != nil {
		return errors.Wrapf(err, "failed to get property \"%s\"", name)
	}
	defer prop.Clear()
	// skip null value
	if prop.raw.VT == ole.VT_NULL {
		return nil
	}
	return setValue(field, prop)
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

// setValue is used to set property to structure field.
func setValue(field reflect.Value, prop *Object) error {
	if !field.CanSet() {
		fieldType := field.Type()
		return &ErrFieldMismatch{
			FieldName:  fieldType.Name(),
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
			fieldType := field.Type()
			return &ErrFieldMismatch{
				FieldName:  fieldType.Name(),
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
			fieldType := field.Type()
			return &ErrFieldMismatch{
				FieldName:  fieldType.Name(),
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
			fieldType := field.Type()
			return &ErrFieldMismatch{
				FieldName:  fieldType.Name(),
				StructType: fieldType,
				Reason:     "not a float type",
			}
		}
	case bool:
		switch field.Kind() {
		case reflect.Bool:
			field.SetBool(val)
		default:
			fieldType := field.Type()
			return &ErrFieldMismatch{
				FieldName:  fieldType.Name(),
				StructType: fieldType,
				Reason:     "not a bool type",
			}
		}
	case string:
		return setStringValue(field, val)
	default:
		return setOtherValue(field, val)
	}
	return nil
}

func setStringValue(field reflect.Value, val string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			fieldType := field.Type()
			return &ErrFieldMismatch{
				FieldName:  fieldType.Name(),
				StructType: fieldType,
				Reason:     err.Error(),
			}
		}
		field.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			fieldType := field.Type()
			return &ErrFieldMismatch{
				FieldName:  fieldType.Name(),
				StructType: fieldType,
				Reason:     err.Error(),
			}
		}
		field.SetUint(u)
	case reflect.Struct: // string to time.Duration
		switch field.Type() {
		case timeType:
			if len(val) == 25 {
				m, err := strconv.Atoi(val[22:])
				if err != nil {
					fieldType := field.Type()
					return &ErrFieldMismatch{
						FieldName:  fieldType.Name(),
						StructType: fieldType,
						Reason:     err.Error(),
					}
				}
				val = val[:22] + fmt.Sprintf("%02d%02d", m/60, m%60)
			}
			t, err := time.Parse("20060102150405.000000-0700", val)
			if err != nil {
				fieldType := field.Type()
				return &ErrFieldMismatch{
					FieldName:  fieldType.Name(),
					StructType: fieldType,
					Reason:     err.Error(),
				}
			}
			field.Set(reflect.ValueOf(t))
		default:
			fieldType := field.Type()
			return &ErrFieldMismatch{
				FieldName:  fieldType.Name(),
				StructType: fieldType,
				Reason:     "not a string to time.Duration",
			}
		}
	default:
		fieldType := field.Type()
		return &ErrFieldMismatch{
			FieldName:  fieldType.Name(),
			StructType: fieldType,
			Reason:     fmt.Sprintf("string can not set to this field"),
		}
	}
	return nil
}

func setOtherValue(field reflect.Value, val interface{}) error {
	switch field.Kind() {
	case reflect.Slice:

	case reflect.Struct:

	default:
		fieldType := field.Type()
		return &ErrFieldMismatch{
			FieldName:  fieldType.Name(),
			StructType: fieldType,
			Reason:     fmt.Sprintf("unsupported type (%T)", val),
		}
	}
	return nil
}

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

// use minutes to replace hour + minute, because time zone is [-12,+14] after
// convert to minute is [-720,+840], so we only need three characters
func timeToWMIDateTime(t time.Time) string {
	str := t.Format("20060102150405.000000-0700")
	hour, err := strconv.ParseInt(str[22:24], 10, 64)
	if err != nil {
		panic("invalid time string after format")
	}
	minute, err := strconv.ParseInt(str[24:26], 10, 64)
	if err != nil {
		panic("invalid time string after format")
	}
	return fmt.Sprintf(str[:22]+"%03d", hour*60+minute)
}

// getStructFields is used to get structure field names, it will process wmi structure tag.
func getStructFields(structure reflect.Type) []string {
	l := structure.NumField()
	fields := make([]string, l)
	for i := 0; i < l; i++ {
		field := structure.Field(i)
		// skip unexported field
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		// check structure tag
		tag, ok := field.Tag.Lookup("wmi")
		if !ok {
			fields[i] = field.Name
			continue
		}
		switch tag {
		case "-":
		case "":
			panic("empty value in wmi tag")
		default:
			fields[i] = tag
		}
	}
	return fields
}

// parseExecQueryResult is used to parse ExecQuery result to destination interface.
func parseExecQueryResult(objects []*Object, dst interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "parseExecQueryResult")
		}
	}()
	val := reflect.ValueOf(dst)
	sliceType, elemType := checkExecQueryDstType(dst, val)
	// make slice
	objectsLen := len(objects)
	val = val.Elem() // slice
	val.Set(reflect.MakeSlice(sliceType, objectsLen, objectsLen))
	// check slice element is pointer
	elemIsPtr := elemType.Kind() == reflect.Ptr
	if elemIsPtr {
		elemType = elemType.Elem()
	}
	fields := getStructFields(elemType)
	for i := 0; i < objectsLen; i++ {
		elem := val.Index(i)
		if elemIsPtr {
			elem.Set(reflect.New(elemType))
			elem = elem.Elem()
		}
		for j := 0; j < len(fields); j++ {
			// skipped field
			if fields[j] == "" {
				continue
			}
			err = setProperty(elem.Field(j), objects[i], fields[j])
			if err != nil {
				return
			}
		}
	}
	return
}

func checkExecQueryDstType(dst interface{}, val reflect.Value) (slice, elem reflect.Type) {
	if dst == nil {
		panic("destination interface is nil")
	}
	typ := reflect.TypeOf(dst)
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("destination interface is not slice pointer or it is nil pointer")
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
func parseExecMethodResult(object *Object, output interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "parseExecMethodResult")
		}
	}()
	if output == nil {
		return
	}
	typ, val := checkExecMethodOutputType(output)
	fields := getStructFields(typ)
	for i := 0; i < len(fields); i++ {
		// skipped field
		if fields[i] == "" {
			continue
		}
		err = setProperty(val.Field(i), object, fields[i])
		if err != nil {
			return
		}
	}
	return
}

func checkExecMethodOutputType(output interface{}) (reflect.Type, reflect.Value) {
	typ := reflect.TypeOf(output)
	val := reflect.ValueOf(output)
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("output interface is not pointer or it is nil pointer")
	}
	elem := typ.Elem()
	if elem.Kind() != reflect.Struct {
		panic("output pointer is not point to structure")
	}
	return elem, val.Elem()
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
	return setValue(field, prop.Value(), prop)
}

// ErrFieldMismatch is returned when a field is to be loaded into a different
// type than the one it was stored from, or when a field is missing or
// unexported in the destination struct.
// StructType is the type of the struct pointed to by the destination argument.
type ErrFieldMismatch struct {
	FieldName  string
	StructType reflect.Type
	Reason     interface{}
}

func (e *ErrFieldMismatch) Error() string {
	const format = "can not set field %q with a %q: %s"
	return fmt.Sprintf(format, e.FieldName, e.StructType, e.Reason)
}

func newErrFieldMismatch(field string, typ reflect.Type, reason interface{}) *ErrFieldMismatch {
	return &ErrFieldMismatch{
		FieldName:  field,
		StructType: typ,
		Reason:     reason,
	}
}

// setValue is used to set property to structure field.
func setValue(field reflect.Value, value interface{}, prop *Object) error {
	if field.Kind() == reflect.Ptr {
		field.Set(reflect.New(field.Type().Elem()))
		field = field.Elem()
	}
	if !field.CanSet() {
		fieldType := field.Type()
		return &ErrFieldMismatch{
			FieldName:  fieldType.Name(),
			StructType: fieldType,
			Reason:     "can not set value",
		}
	}
	switch val := value.(type) {
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
		return setOtherValue(field, val, prop)
	}
	return nil
}

var timeType = reflect.TypeOf(time.Time{})

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
			Reason:     "string can not set to this field",
		}
	}
	return nil
}

func setOtherValue(field reflect.Value, val interface{}, prop *Object) error {
	switch field.Kind() {
	case reflect.Slice:
		values := prop.ToArray()
		l := len(values)
		field.Set(reflect.MakeSlice(field.Type(), l, l))
		slice := field
		elemType := slice.Type().Elem()
		// check slice element is pointer
		elemIsPtr := elemType.Kind() == reflect.Ptr
		if elemIsPtr {
			elemType = elemType.Elem()
		}
		for i := 0; i < l; i++ {
			elem := slice.Index(i)
			if elemIsPtr {
				elem.Set(reflect.New(elemType))
				elem = elem.Elem()
			}
			err := setValue(elem, values[i], prop)
			if err != nil {
				return err
			}
		}
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

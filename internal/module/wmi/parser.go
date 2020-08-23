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

var timeType = reflect.TypeOf(time.Time{})

// use minutes to replace hour + minute, because time zone is [-12,+14] after
// convert to minute is [-720,+840], so we only need three characters
func timeToWMIDateTime(t time.Time) string {
	str := t.Format("20060102150405.000000-0700")
	hour, _ := strconv.ParseInt(str[22:24], 10, 64)
	minute, _ := strconv.ParseInt(str[24:26], 10, 64)
	return fmt.Sprintf(str[:22]+"%03d", hour*60+minute)
}

func wmiDateTimeToTime(str string) (time.Time, error) {
	if len(str) != 25 {
		return time.Time{}, errors.New("invalid date time string")
	}
	minute, err := strconv.Atoi(str[22:])
	if err != nil {
		return time.Time{}, err
	}
	str = str[:22] + fmt.Sprintf("%02d%02d", minute/60, minute%60)
	return time.Parse("20060102150405.000000-0700", str)
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
	if dst == nil {
		panic("destination interface is nil")
	}
	// check destination type
	typ := reflect.TypeOf(dst)
	val := reflect.ValueOf(dst)
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("destination interface is not slice pointer or it is nil pointer")
	}
	sliceType := typ.Elem()
	if sliceType.Kind() != reflect.Slice {
		panic("destination pointer is not point to slice")
	}
	// check slice element type
	elemType := sliceType.Elem()
	var elemIsPtr bool
	switch elemType.Kind() {
	case reflect.Struct:
	case reflect.Ptr:
		if elemType.Elem().Kind() != reflect.Struct {
			panic("destination slice element pointer is not point to structure")
		}
		elemIsPtr = true
		elemType = elemType.Elem()
	default:
		panic("destination slice element is not structure or pointer")
	}
	// make slice
	val = val.Elem()
	l := len(objects)
	val.Set(reflect.MakeSlice(sliceType, l, l))
	for i := 0; i < l; i++ {
		elem := val.Index(i)
		if elemIsPtr {
			if elem.IsNil() {
				elem.Set(reflect.New(elemType))
			}
			elem = elem.Elem()
		}
		err = walkStruct(objects[i], elemType, elem)
		if err != nil {
			return
		}
	}
	return
}

// parseExecMethodOutput is used to parse ExecMethod result to destination interface.
func parseExecMethodOutput(object *Object, output interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "parseExecMethodOutput")
		}
	}()
	if output == nil {
		return
	}
	// check output type
	typ := reflect.TypeOf(output)
	val := reflect.ValueOf(output)
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("output interface is not pointer or it is nil pointer")
	}
	typ = typ.Elem()
	if typ.Kind() != reflect.Struct {
		panic("output pointer is not point to structure")
	}
	val = val.Elem()
	return walkStruct(object, typ, val)
}

func walkStruct(obj *Object, typ reflect.Type, dst reflect.Value) error {
	fields := getStructFields(typ)
	for i := 0; i < len(fields); i++ {
		// skipped field
		if fields[i] == "" {
			continue
		}
		err := getProperty(obj, fields[i], typ.Field(i).Type, dst.Field(i))
		if err != nil {
			return err
		}
	}
	return nil
}

func getProperty(obj *Object, name string, typ reflect.Type, dst reflect.Value) error {
	prop, err := obj.GetProperty(name)
	if err != nil {
		return errors.Wrapf(err, "failed to get property %q", name)
	}
	defer prop.Clear()
	if prop.raw.VT == ole.VT_NULL { // skip null value
		return nil
	}
	return setValue(name, typ, dst, prop.Value(), prop)
}

// ErrFieldMismatch is returned when a field is to be loaded into a different
// type than the one it was stored from, or when a field is missing or unexported
// in the destination struct.
type ErrFieldMismatch struct {
	Name   string
	Type   reflect.Type
	Reason interface{}
}

func (e *ErrFieldMismatch) Error() string {
	const format = "can not set field %q with a %s type: %s"
	return fmt.Sprintf(format, e.Name, e.Type, e.Reason)
}

func newErrFieldMismatch(name string, typ reflect.Type, reason interface{}) *ErrFieldMismatch {
	return &ErrFieldMismatch{
		Name:   name,
		Type:   typ,
		Reason: reason,
	}
}

// setValue is used to set property to destination value.
func setValue(name string, typ reflect.Type, dst reflect.Value, val interface{}, prop *Object) error {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		dst.Set(reflect.New(typ))
		dst = dst.Elem()
	}
	switch val := val.(type) {
	case int, int8, int16, int32, int64:
		return setIntValue(name, typ, dst, reflect.ValueOf(val).Int())
	case uint, uint8, uint16, uint32, uint64:
		return setUintValue(name, typ, dst, reflect.ValueOf(val).Uint())
	case float32, float64:
		return setFloatValue(name, typ, dst, reflect.ValueOf(val).Float())
	case bool:
		return setBoolValue(name, typ, dst, val)
	case string:
		return setStringValue(name, typ, dst, val)
	default:
		return setOtherValue(name, typ, dst, prop)
	}
}

func setIntValue(name string, typ reflect.Type, dst reflect.Value, val int64) error {
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		dst.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		dst.SetUint(uint64(val))
	default:
		return newErrFieldMismatch(name, typ, "not an integer type")
	}
	return nil
}

func setUintValue(name string, typ reflect.Type, dst reflect.Value, val uint64) error {
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		dst.SetInt(int64(val))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		dst.SetUint(val)
	default:
		return newErrFieldMismatch(name, typ, "not an unsigned integer type")
	}
	return nil
}

func setFloatValue(name string, typ reflect.Type, dst reflect.Value, val float64) error {
	switch typ.Kind() {
	case reflect.Float32, reflect.Float64:
		dst.SetFloat(val)
	default:
		return newErrFieldMismatch(name, typ, "not a float type")
	}
	return nil
}

func setBoolValue(name string, typ reflect.Type, dst reflect.Value, val bool) error {
	switch typ.Kind() {
	case reflect.Bool:
		dst.SetBool(val)
	default:
		return newErrFieldMismatch(name, typ, "not a bool type")
	}
	return nil
}

func setStringValue(name string, typ reflect.Type, dst reflect.Value, val string) error {
	switch typ.Kind() {
	case reflect.String:
		dst.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return newErrFieldMismatch(name, typ, err)
		}
		dst.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return newErrFieldMismatch(name, typ, err)
		}
		dst.SetUint(u)
	case reflect.Struct: // string to time.Time
		switch typ {
		case timeType:
			t, err := wmiDateTimeToTime(val)
			if err != nil {
				return newErrFieldMismatch(name, typ, err)
			}
			dst.Set(reflect.ValueOf(t))
		default:
			return newErrFieldMismatch(name, typ, "not a string to time.Time")
		}
	default:
		return newErrFieldMismatch(name, typ, "string can not set to this value")
	}
	return nil
}

func setOtherValue(name string, typ reflect.Type, dst reflect.Value, prop *Object) error {
	switch typ.Kind() {
	case reflect.Slice:
		values := prop.ToArray()
		l := len(values)
		dst.Set(reflect.MakeSlice(dst.Type(), l, l))
		for i := 0; i < l; i++ {
			elem := dst.Index(i)
			err := setValue(name, elem.Type(), elem, values[i], prop)
			if err != nil {
				return err
			}
		}
	case reflect.Struct:
		return walkStruct(prop, typ, dst)
	default:
		return newErrFieldMismatch(name, typ, "unsupported type")
	}
	return nil
}

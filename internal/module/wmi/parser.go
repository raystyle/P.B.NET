// +build windows

package wmi

import (
	"reflect"

	"github.com/go-ole/go-ole"

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
			if fields[j] == "" {
				continue
			}

			prop, err := object.GetProperty(fields[j])
			if err != nil {
				return err
			}

			if prop.raw.VT == ole.VT_NULL {
				continue
			}

			switch val := prop.Value().(type) {
			case int32:

				element.Field(j).SetUint(uint64(val))
			case string:
				element.Field(j).SetString(val)
			default:
			}

			prop.Clear()
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

package xreflect

import (
	"reflect"
	"strings"
)

// GetStructureName is used to get the structure name.
func GetStructureName(v interface{}) string {
	// package name.structure name
	s := reflect.TypeOf(v).String()
	ss := strings.Split(s, ".")
	return ss[len(ss)-1]
}

// StructureToMap is used to convert structure to a string map.
func StructureToMap(v interface{}, tag string) map[string]interface{} {
	typ, value := structureToMap(v)
	n := value.NumField()
	m := make(map[string]interface{}, n)
	for i := 0; i < n; i++ {
		key := typ.Field(i).Tag.Get(tag)
		value := value.Field(i).Interface()
		m[key] = value
	}
	return m
}

// StructureToMapWithoutZero is used to convert structure to a string map but not
// include zero value like 0, false, "".
func StructureToMapWithoutZero(v interface{}, tag string) map[string]interface{} {
	typ, value := structureToMap(v)
	n := value.NumField()
	m := make(map[string]interface{}, n)
	for i := 0; i < n; i++ {
		key := typ.Field(i).Tag.Get(tag)
		value := value.Field(i)
		if !value.IsZero() {
			m[key] = value.Interface()
		}
	}
	return m
}

func structureToMap(v interface{}) (reflect.Type, reflect.Value) {
	typ := reflect.TypeOf(v)
	var value reflect.Value
	if typ.Kind() == reflect.Ptr {
		value = reflect.ValueOf(v).Elem()
		typ = value.Type()
	} else {
		value = reflect.ValueOf(v)
	}
	return typ, value
}

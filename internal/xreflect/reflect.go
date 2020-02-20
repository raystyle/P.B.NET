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

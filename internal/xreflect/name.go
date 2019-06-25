package xreflect

import (
	"reflect"
	"strings"
)

func Struct_Name(v interface{}) string {
	s := reflect.TypeOf(v).String()
	ss := strings.Split(s, ".")
	return ss[len(ss)-1]
}

package xreflect

import (
	"reflect"
	"strings"
)

func StructName(v interface{}) string {
	s := reflect.TypeOf(v).String()
	ss := strings.Split(s, ".")
	return ss[len(ss)-1]
}

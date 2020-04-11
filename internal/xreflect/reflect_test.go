package xreflect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testStruct struct{}

func TestGetStructureName(t *testing.T) {
	name := GetStructureName(&testStruct{})
	require.Equal(t, "testStruct", name)
	nest := struct {
		a int
		b struct {
			c int
			d int
		}
	}{}
	name = GetStructureName(nest)
	expected := "struct { a int; b struct { c int; d int } }"
	require.Equal(t, expected, name)
}

func TestStructureToMap(t *testing.T) {
	s := struct {
		Name string `msgpack:"name"`
		Host string `msgpack:"host"`
	}{
		Name: "aaa",
		Host: "bbb",
	}
	// point
	m := StructureToMap(&s, "msgpack")
	require.Equal(t, "aaa", m["name"])
	require.Equal(t, "bbb", m["host"])
	// value
	m = StructureToMap(s, "msgpack")
	require.Equal(t, "aaa", m["name"])
	require.Equal(t, "bbb", m["host"])
}

func TestStructureToMapWithoutZero(t *testing.T) {
	s := struct {
		Name string `msgpack:"name"`
		Host string `msgpack:"host"`
	}{
		Name: "aaa",
		Host: "",
	}
	// point
	m := StructureToMapWithoutZero(&s, "msgpack")
	require.Len(t, m, 1)
	require.Equal(t, "aaa", m["name"])
	// value
	m = StructureToMapWithoutZero(s, "msgpack")
	require.Len(t, m, 1)
	require.Equal(t, "aaa", m["name"])
}

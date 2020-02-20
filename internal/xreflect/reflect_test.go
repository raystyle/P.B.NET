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

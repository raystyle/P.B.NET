package xreflect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testStruct struct{}

func Test_Struct_Name(t *testing.T) {
	name := StructName(&testStruct{})
	require.Equal(t, "testStruct", name)
}

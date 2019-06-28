package xreflect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type test_struct struct{}

func Test_Struct_Name(t *testing.T) {
	name := Struct_Name(&test_struct{})
	require.Equal(t, "test_struct", name)
}

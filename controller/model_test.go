package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testStruct struct{}

func TestGetStructureName(t *testing.T) {
	name := getStructureName(&testStruct{})
	require.Equal(t, "testStruct", name)
}

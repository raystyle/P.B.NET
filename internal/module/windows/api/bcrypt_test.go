package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBCryptOpenAlgorithmProvider(t *testing.T) {
	handle, err := BCryptOpenAlgorithmProvider("3DES", "", 0)
	require.NoError(t, err)

	fmt.Println(handle)

	err = BCryptCloseAlgorithmProvider(handle, 0)
	require.NoError(t, err)
}

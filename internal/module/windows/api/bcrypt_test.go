package api

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestBCryptOpenAlgorithmProvider(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle, err := BCryptOpenAlgorithmProvider("3DES", "", 0)
		require.NoError(t, err)

		t.Log(handle)
	})
}

func TestBCryptCloseAlgorithmProvider(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle, err := BCryptOpenAlgorithmProvider("3DES", "", 0)
		require.NoError(t, err)

		err = BCryptCloseAlgorithmProvider(handle, 0)
		require.NoError(t, err)
	})
}

func testBCryptOpenAlgorithmProvider(t *testing.T) BcryptHandle {
	handle, err := BCryptOpenAlgorithmProvider("3DES", "", 0)
	require.NoError(t, err)
	return handle
}

func testBCryptCloseAlgorithmProvider(t *testing.T, handle BcryptHandle) {
	err := BCryptCloseAlgorithmProvider(handle, 0)
	require.NoError(t, err)
}

func TestBCryptSetProperty(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle := testBCryptOpenAlgorithmProvider(t)

		prop := "ChainingMode"
		mode := windows.StringToUTF16("ChainingModeCBC")
		err := BCryptSetProperty(handle, prop, (*byte)(unsafe.Pointer(&mode[0])), uint32(len(mode)), 0)
		require.NoError(t, err)

		testBCryptCloseAlgorithmProvider(t, handle)
	})
}

func TestBCryptGetProperty(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handle := testBCryptOpenAlgorithmProvider(t)

		prop := "ChainingMode"
		mode := windows.StringToUTF16("ChainingModeCBC")
		err := BCryptSetProperty(handle, prop, (*byte)(unsafe.Pointer(&mode[0])), uint32(len(mode)), 0)
		require.NoError(t, err)

		prop = "ObjectLength"
		var length uint32
		result, err := BCryptGetProperty(handle, prop, (*byte)(unsafe.Pointer(&length)), 4, 0)
		require.NoError(t, err)
		require.Equal(t, uint32(4), result)

		t.Log(length)

		testBCryptCloseAlgorithmProvider(t, handle)
	})
}

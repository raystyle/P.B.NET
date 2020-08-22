// +build windows

package wmi

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestObject_AddProperty(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testCreateClient(t)

	object, err := client.GetObject("Win32_ProcessStartup")
	require.NoError(t, err)

	t.Run("int8", func(t *testing.T) {
		const name = "int8"

		err := object.AddProperty(name, CIMTypeInt8, false)
		require.NoError(t, err)

		err = object.SetProperty(name, int8(math.MaxInt8))
		require.NoError(t, err)
		val1, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MaxInt8, val1.Value())

		err = object.SetProperty(name, int8(math.MinInt8))
		require.NoError(t, err)
		val2, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MinInt8, val2.Value())

		err = object.RemoveProperty(name)
		require.NoError(t, err)
	})

	t.Run("int16", func(t *testing.T) {
		const name = "int16"

		err := object.AddProperty(name, CIMTypeInt16, false)
		require.NoError(t, err)

		err = object.SetProperty(name, int16(math.MaxInt16))
		require.NoError(t, err)
		val1, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MaxInt16, val1.Value())

		err = object.SetProperty(name, int16(math.MinInt16))
		require.NoError(t, err)
		val2, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MinInt16, val2.Value())

		err = object.RemoveProperty(name)
		require.NoError(t, err)
	})

	t.Run("array", func(t *testing.T) {

	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

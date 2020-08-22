// +build windows

package wmi

import (
	"math"
	"strconv"
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

	t.Run("int32", func(t *testing.T) {
		const name = "int32"

		err := object.AddProperty(name, CIMTypeInt32, false)
		require.NoError(t, err)

		err = object.SetProperty(name, int32(math.MaxInt32))
		require.NoError(t, err)
		val1, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MaxInt32, val1.Value())

		err = object.SetProperty(name, int32(math.MinInt32))
		require.NoError(t, err)
		val2, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MinInt32, val2.Value())

		err = object.RemoveProperty(name)
		require.NoError(t, err)
	})

	t.Run("int64", func(t *testing.T) {
		const name = "int64"

		err := object.AddProperty(name, CIMTypeInt64, false)
		require.NoError(t, err)

		err = object.SetProperty(name, int64(math.MaxInt64))
		require.NoError(t, err)
		val1, err := object.GetProperty(name)
		require.NoError(t, err)
		v1, err := strconv.ParseInt(val1.Value().(string), 10, 64)
		require.NoError(t, err)
		require.EqualValues(t, math.MaxInt64, v1)

		err = object.SetProperty(name, int64(math.MinInt64))
		require.NoError(t, err)
		val2, err := object.GetProperty(name)
		require.NoError(t, err)
		v2, err := strconv.ParseInt(val2.Value().(string), 10, 64)
		require.NoError(t, err)
		require.EqualValues(t, math.MinInt64, v2)

		err = object.RemoveProperty(name)
		require.NoError(t, err)
	})

	t.Run("uint8", func(t *testing.T) {
		const name = "uint8"

		err := object.AddProperty(name, CIMTypeUint8, false)
		require.NoError(t, err)

		err = object.SetProperty(name, uint8(math.MaxUint8))
		require.NoError(t, err)
		val1, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MaxUint8, val1.Value())

		err = object.SetProperty(name, uint8(0))
		require.NoError(t, err)
		val2, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, 0, val2.Value())

		err = object.RemoveProperty(name)
		require.NoError(t, err)
	})

	t.Run("uint16", func(t *testing.T) {
		const name = "uint16"

		err := object.AddProperty(name, CIMTypeUint16, false)
		require.NoError(t, err)

		err = object.SetProperty(name, uint16(math.MaxUint16))
		require.NoError(t, err)
		val1, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MaxUint16, val1.Value())

		err = object.SetProperty(name, uint16(0))
		require.NoError(t, err)
		val2, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, 0, val2.Value())

		err = object.RemoveProperty(name)
		require.NoError(t, err)
	})

	t.Run("uint32", func(t *testing.T) {
		const name = "uint32"

		err := object.AddProperty(name, CIMTypeUint32, false)
		require.NoError(t, err)

		err = object.SetProperty(name, uint32(math.MaxUint32))
		require.NoError(t, err)
		val1, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, math.MaxUint32, val1.Value())

		err = object.SetProperty(name, uint32(0))
		require.NoError(t, err)
		val2, err := object.GetProperty(name)
		require.NoError(t, err)
		require.EqualValues(t, 0, val2.Value())

		err = object.RemoveProperty(name)
		require.NoError(t, err)
	})

	t.Run("array", func(t *testing.T) {

	})

	client.Close()

	testsuite.IsDestroyed(t, client)
}

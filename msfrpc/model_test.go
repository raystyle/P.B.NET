package msfrpc

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/vmihailenco/msgpack/v5/msgpcode"

	"project/internal/patch/monkey"
)

func TestErrorCode_DecodeMsgpack(t *testing.T) {
	t.Run("peek code", func(t *testing.T) {
		data := []byte("")
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		var errCode errorCode
		err := errCode.DecodeMsgpack(decoder)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("uint16", func(t *testing.T) {
		data := []byte{msgpcode.Uint16, 0x00, 0x01}
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		var errCode errorCode
		err := errCode.DecodeMsgpack(decoder)
		require.NoError(t, err)
		require.Equal(t, uint64(1), uint64(errCode))
	})

	t.Run("invalid uint16", func(t *testing.T) {
		data := []byte{msgpcode.Uint16, 0x00}
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		var errCode errorCode
		err := errCode.DecodeMsgpack(decoder)
		require.Equal(t, io.ErrUnexpectedEOF, err)
	})

	t.Run("bin8", func(t *testing.T) {
		// type | data | data
		data := []byte{msgpcode.Bin8, 0x01, []byte("1")[0]}
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		var errCode errorCode
		err := errCode.DecodeMsgpack(decoder)
		require.NoError(t, err)
		require.Equal(t, uint64(1), uint64(errCode))
	})

	t.Run("invalid bin8", func(t *testing.T) {
		// type | data | data
		data := []byte{msgpcode.Bin8, 0x02, []byte("1")[0]}
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		var errCode errorCode
		err := errCode.DecodeMsgpack(decoder)
		require.Equal(t, io.ErrUnexpectedEOF, err)
	})

	t.Run("invalid bin8 NaN", func(t *testing.T) {
		// type | data | data
		data := []byte{msgpcode.Bin8, 0x01, []byte("a")[0]}
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		var errCode errorCode
		err := errCode.DecodeMsgpack(decoder)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("unknown code", func(t *testing.T) {
		data := []byte("foo data")
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		var errCode errorCode
		err := errCode.DecodeMsgpack(decoder)
		require.Error(t, err)
		t.Log(err)
	})
}

func TestMSFError_Error(t *testing.T) {
	msfErr := &MSFError{ErrorMessage: "test"}
	require.EqualError(t, msfErr, "test")
}

func TestLicense_DecodeMsgpack(t *testing.T) {
	t.Run("peek code", func(t *testing.T) {
		data := []byte("")
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		license := new(license)
		err := license.DecodeMsgpack(decoder)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("bin", func(t *testing.T) {
		data := []byte{msgpcode.Bin8}
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		license := new(license)
		err := license.DecodeMsgpack(decoder)
		require.Error(t, err)
		t.Log(err)
	})

	t.Run("fixed array", func(t *testing.T) {
		data := []byte{msgpcode.FixedArrayLow}
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		patch := func(interface{}) ([]interface{}, error) {
			return nil, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(decoder, "DecodeSlice", patch)
		defer pg.Unpatch()

		license := new(license)
		err := license.DecodeMsgpack(decoder)
		monkey.IsMonkeyError(t, err)
	})

	t.Run("unknown code", func(t *testing.T) {
		data := []byte("foo data")
		decoder := msgpack.NewDecoder(bytes.NewReader(data))

		license := new(license)
		err := license.DecodeMsgpack(decoder)
		require.Error(t, err)
		t.Log(err)
	})
}

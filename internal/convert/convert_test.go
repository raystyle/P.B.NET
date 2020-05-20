package convert

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNumberToBytes(t *testing.T) {
	if !bytes.Equal(Int16ToBytes(int16(0x0102)), []byte{1, 2}) {
		t.Fatal("Int16ToBytes() with invalid number")
	}
	if !bytes.Equal(Int32ToBytes(int32(0x01020304)), []byte{1, 2, 3, 4}) {
		t.Fatal("Int32ToBytes() with invalid number")
	}
	if !bytes.Equal(Int64ToBytes(int64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("Int16ToBytes() with invalid number")
	}
	if !bytes.Equal(Uint16ToBytes(uint16(0x0102)), []byte{1, 2}) {
		t.Fatal("Uint16ToBytes() with invalid number")
	}
	if !bytes.Equal(Uint32ToBytes(uint32(0x01020304)), []byte{1, 2, 3, 4}) {
		t.Fatal("Uint32ToBytes() with invalid number")
	}
	if !bytes.Equal(Uint64ToBytes(uint64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("Uint64ToBytes() with invalid number")
	}
	if !bytes.Equal(Float32ToBytes(float32(123.123)), []byte{66, 246, 62, 250}) {
		t.Fatal("Float32ToBytes() with invalid number")
	}
	if !bytes.Equal(Float64ToBytes(123.123), []byte{64, 94, 199, 223, 59, 100, 90, 29}) {
		t.Fatal("Float64ToBytes() with invalid number")
	}
}

func TestBytesToNumber(t *testing.T) {
	if BytesToInt16([]byte{1, 2}) != 0x0102 {
		t.Fatal("BytesToInt16() with invalid bytes")
	}
	if BytesToInt32([]byte{1, 2, 3, 4}) != 0x01020304 {
		t.Fatal("BytesToInt32() with invalid bytes")
	}
	if BytesToInt64([]byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0x0102030405060708 {
		t.Fatal("BytesToInt64() with invalid bytes")
	}
	if BytesToUint16([]byte{1, 2}) != 0x0102 {
		t.Fatal("BytesToUint16() with invalid bytes")
	}
	if BytesToUint32([]byte{1, 2, 3, 4}) != 0x01020304 {
		t.Fatal("BytesToUint32() with invalid bytes")
	}
	if BytesToUint64([]byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0x0102030405060708 {
		t.Fatal("BytesToUint64() with invalid bytes")
	}
	if BytesToFloat32([]byte{66, 246, 62, 250}) != 123.123 {
		t.Fatal("BytesToFloat32() with invalid bytes")
	}
	if BytesToFloat64([]byte{64, 94, 199, 223, 59, 100, 90, 29}) != 123.123 {
		t.Fatal("BytesToFloat64() with invalid bytes")
	}
}

func TestBytesToNumberWithInvalidBytes(t *testing.T) {
	def := func() {
		r := recover()
		require.NotNil(t, r)
	}

	t.Run("BytesToInt16", func(t *testing.T) {
		defer def()
		BytesToInt16([]byte{1})
	})

	t.Run("BytesToInt32", func(t *testing.T) {
		defer def()
		BytesToInt32([]byte{1})
	})

	t.Run("BytesToInt64", func(t *testing.T) {
		defer def()
		BytesToInt64([]byte{1})
	})

	t.Run("BytesToUint16", func(t *testing.T) {
		defer def()
		BytesToUint16([]byte{1})
	})

	t.Run("BytesToUint32", func(t *testing.T) {
		defer def()
		BytesToUint32([]byte{1})
	})

	t.Run("BytesToUint64", func(t *testing.T) {
		defer def()
		BytesToUint64([]byte{1})
	})

	t.Run("BytesToFloat32", func(t *testing.T) {
		defer def()
		BytesToFloat32([]byte{1})
	})

	t.Run("BytesToFloat64", func(t *testing.T) {
		defer def()
		BytesToFloat64([]byte{1})
	})

	// negative number
	n := int64(-0x12345678)
	if BytesToInt64(Int64ToBytes(n)) != n {
		t.Fatal("negative number")
	}
}

func TestAbsInt64(t *testing.T) {
	testdata := [...]*struct {
		input  int64
		output int64
	}{
		{-1, 1},
		{0, 0},
		{1, 1},
		{-10, 10},
		{10, 10},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].output, AbsInt64(testdata[i].input))
	}
}

func TestByteToString(t *testing.T) {
	testdata := [...]*struct {
		input  int
		output string
	}{
		{1023, "1023 Byte"},
		{1024, "1.000 KB"},
		{1536, "1.500 KB"},
		{1024 << 10, "1.000 MB"},
		{1536 << 10, "1.500 MB"},
		{1024 << 20, "1.000 GB"},
		{1536 << 20, "1.500 GB"},
		{1024 << 30, "1.000 TB"},
		{1536 << 30, "1.500 TB"},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].output, ByteToString(uint64(testdata[i].input)))
	}
}

func TestFormatNumber(t *testing.T) {
	testdata := [...]*struct {
		input  string
		output string
	}{
		{"1", "1"},
		{"12", "12"},
		{"123", "123"},
		{"1234", "1,234"},
		{"12345", "12,345"},
		{"123456", "123,456"},
		{"1234567", "1,234,567"},
		{"12345678", "12,345,678"},
		{"123456789", "123,456,789"},
		{"123456789.1", "123,456,789.1"},
		{"123456789.12", "123,456,789.12"},
		{"123456789.123", "123,456,789.123"},
		{"123456789.1234", "123,456,789.1234"},
		{"0.123", "0.123"},
		{"0.1234", "0.1234"},
		{".1234", ".1234"},
		{".12", ".12"},
		{"0.123456", "0.123456"},
		{"123456.789", "123,456.789"},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].output, FormatNumber(testdata[i].input))
	}
}

func TestByteSliceToString(t *testing.T) {
	testdata := [...]*struct {
		input  []byte
		output string
	}{
		{[]byte{}, "[]byte{}"},
		{[]byte{1}, "[]byte{1}"},
		{[]byte{1, 2}, "[]byte{1, 2}"},
		{[]byte{1, 2, 3}, "[]byte{1, 2, 3}"},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].output, ByteSliceToString(testdata[i].input))
	}
}

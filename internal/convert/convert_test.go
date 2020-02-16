package convert

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNumberToBytes(t *testing.T) {
	if bytes.Compare(Int16ToBytes(int16(0x0102)), []byte{1, 2}) != 0 {
		t.Fatal("Int16ToBytes() invalid number")
	}
	if bytes.Compare(Int32ToBytes(int32(0x01020304)), []byte{1, 2, 3, 4}) != 0 {
		t.Fatal("Int32ToBytes() invalid number")
	}
	if bytes.Compare(Int64ToBytes(int64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0 {
		t.Fatal("Int16ToBytes() invalid number")
	}
	if bytes.Compare(Uint16ToBytes(uint16(0x0102)), []byte{1, 2}) != 0 {
		t.Fatal("Uint16ToBytes() invalid number")
	}
	if bytes.Compare(Uint32ToBytes(uint32(0x01020304)), []byte{1, 2, 3, 4}) != 0 {
		t.Fatal("Uint32ToBytes() invalid number")
	}
	if bytes.Compare(Uint64ToBytes(uint64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0 {
		t.Fatal("Uint64ToBytes() invalid number")
	}
	if bytes.Compare(Float32ToBytes(float32(123.123)), []byte{66, 246, 62, 250}) != 0 {
		t.Fatal("Float32ToBytes() invalid number")
	}
	if bytes.Compare(Float64ToBytes(123.123), []byte{64, 94, 199, 223, 59, 100, 90, 29}) != 0 {
		t.Fatal("Float64ToBytes() invalid number")
	}
}

func TestBytesToNumber(t *testing.T) {
	if BytesToInt16([]byte{1, 2}) != 0x0102 {
		t.Fatal("BytesToInt16() invalid bytes")
	}
	if BytesToInt32([]byte{1, 2, 3, 4}) != 0x01020304 {
		t.Fatal("BytesToInt32() invalid bytes")
	}
	if BytesToInt64([]byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0x0102030405060708 {
		t.Fatal("BytesToInt64() invalid bytes")
	}
	if BytesToUint16([]byte{1, 2}) != 0x0102 {
		t.Fatal("BytesToUint16() invalid bytes")
	}
	if BytesToUint32([]byte{1, 2, 3, 4}) != 0x01020304 {
		t.Fatal("BytesToUint32() invalid bytes")
	}
	if BytesToUint64([]byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0x0102030405060708 {
		t.Fatal("BytesToUint64() invalid bytes")
	}
	if BytesToFloat32([]byte{66, 246, 62, 250}) != 123.123 {
		t.Fatal("BytesToFloat32() invalid bytes")
	}
	if BytesToFloat64([]byte{64, 94, 199, 223, 59, 100, 90, 29}) != 123.123 {
		t.Fatal("BytesToFloat64() invalid bytes")
	}
}

func TestBytesToNumberWithInvalidBytes(t *testing.T) {
	if BytesToInt16([]byte{1}) != 0 {
		t.Fatal("BytesToInt16() invalid bytes & result")
	}
	if BytesToInt32([]byte{1}) != 0 {
		t.Fatal("BytesToInt32() invalid bytes & result")
	}
	if BytesToInt64([]byte{1}) != 0 {
		t.Fatal("BytesToInt64() invalid bytes & result")
	}
	if BytesToUint16([]byte{1}) != 0 {
		t.Fatal("BytesToUint16() invalid bytes & result")
	}
	if BytesToUint32([]byte{1}) != 0 {
		t.Fatal("BytesToUint32() invalid bytes & result")
	}
	if BytesToUint64([]byte{1}) != 0 {
		t.Fatal("BytesToUint64() invalid bytes & result")
	}
	if BytesToFloat32([]byte{1}) != 0 {
		t.Fatal("BytesToFloat32() invalid bytes & result")
	}
	if BytesToFloat64([]byte{1}) != 0 {
		t.Fatal("BytesToFloat64() invalid bytes & result")
	}
	// negative number
	n := int64(-0x12345678)
	if BytesToInt64(Int64ToBytes(n)) != n {
		t.Fatal("negative number")
	}
}

func TestByteToString(t *testing.T) {
	testdata := []*struct {
		except string
		actual int
	}{
		{"1023 Byte", 1023},
		{"1.000 KB", 1024},
		{"1.500 KB", 1536},
		{"1.000 MB", 1024 * 1 << 10},
		{"1.500 MB", 1536 * 1 << 10},
		{"1.000 GB", 1024 * 1 << 20},
		{"1.500 GB", 1536 * 1 << 20},
		{"1.000 TB", 1024 * 1 << 30},
		{"1.500 TB", 1536 * 1 << 30},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].except, ByteToString(uint64(testdata[i].actual)))
	}
}

func TestFormatNumber(t *testing.T) {
	testdata := []*struct {
		input  string
		expect string
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
		require.Equal(t, testdata[i].expect, FormatNumber(testdata[i].input))
	}
}

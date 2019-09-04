package convert

import (
	"bytes"
	"testing"
)

func TestConvert(t *testing.T) {
	if !bytes.Equal(Int16ToBytes(int16(0x0102)), []byte{1, 2}) {
		t.Fatal("Int16ToBytes() invalid number")
	}
	if !bytes.Equal(Int32ToBytes(int32(0x01020304)), []byte{1, 2, 3, 4}) {
		t.Fatal("Int32ToBytes() invalid number")
	}
	if !bytes.Equal(Int64ToBytes(int64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("Int16ToBytes() invalid number")
	}
	if !bytes.Equal(Uint16ToBytes(uint16(0x0102)), []byte{1, 2}) {
		t.Fatal("Uint16ToBytes() invalid number")
	}
	if !bytes.Equal(Uint32ToBytes(uint32(0x01020304)), []byte{1, 2, 3, 4}) {
		t.Fatal("Uint32ToBytes() invalid number")
	}
	if !bytes.Equal(Uint64ToBytes(uint64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("Uint64ToBytes() invalid number")
	}
	if !bytes.Equal(Float32ToBytes(float32(123.123)), []byte{66, 246, 62, 250}) {
		t.Fatal("Float32ToBytes() invalid number")
	}
	if !bytes.Equal(Float64ToBytes(123.123), []byte{64, 94, 199, 223, 59, 100, 90, 29}) {
		t.Fatal("Float64ToBytes() invalid number")
	}
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
	// wrong
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
	// -
	n := int64(-0x12345678)
	if BytesToInt64(Int64ToBytes(n)) != n {
		t.Fatal("wrong n")
	}
}

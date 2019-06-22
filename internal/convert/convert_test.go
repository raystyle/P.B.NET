package convert

import (
	"bytes"
	"testing"
)

func Test_Convert(t *testing.T) {
	if !bytes.Equal(Int16_Bytes(int16(0x0102)), []byte{1, 2}) {
		t.Fatal("Int16_Bytes() invalid number")
	}
	if !bytes.Equal(Int32_Bytes(int32(0x01020304)), []byte{1, 2, 3, 4}) {
		t.Fatal("Int32_Bytes() invalid number")
	}
	if !bytes.Equal(Int64_Bytes(int64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("Int16_Bytes() invalid number")
	}
	if !bytes.Equal(Uint16_Bytes(uint16(0x0102)), []byte{1, 2}) {
		t.Fatal("Uint16_Bytes() invalid number")
	}
	if !bytes.Equal(Uint32_Bytes(uint32(0x01020304)), []byte{1, 2, 3, 4}) {
		t.Fatal("Uint32_Bytes() invalid number")
	}
	if !bytes.Equal(Uint64_Bytes(uint64(0x0102030405060708)), []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal("Uint64_Bytes() invalid number")
	}
	if !bytes.Equal(Float32_Bytes(float32(123.123)), []byte{66, 246, 62, 250}) {
		t.Fatal("Float32_Bytes() invalid number")
	}
	if !bytes.Equal(Float64_Bytes(float64(123.123)), []byte{64, 94, 199, 223, 59, 100, 90, 29}) {
		t.Fatal("Float64_Bytes() invalid number")
	}
	if Bytes_Int16([]byte{1, 2}) != 0x0102 {
		t.Fatal("Bytes_Int16() invalid bytes")
	}
	if Bytes_Int32([]byte{1, 2, 3, 4}) != 0x01020304 {
		t.Fatal("Bytes_Int32() invalid bytes")
	}
	if Bytes_Int64([]byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0x0102030405060708 {
		t.Fatal("Bytes_Int64() invalid bytes")
	}
	if Bytes_Uint16([]byte{1, 2}) != 0x0102 {
		t.Fatal("Bytes_Uint16() invalid bytes")
	}
	if Bytes_Uint32([]byte{1, 2, 3, 4}) != 0x01020304 {
		t.Fatal("Bytes_Uint32() invalid bytes")
	}
	if Bytes_Uint64([]byte{1, 2, 3, 4, 5, 6, 7, 8}) != 0x0102030405060708 {
		t.Fatal("Bytes_Uint64() invalid bytes")
	}
	if Bytes_Float32([]byte{66, 246, 62, 250}) != 123.123 {
		t.Fatal("Bytes_Float32() invalid bytes")
	}
	if Bytes_Float64([]byte{64, 94, 199, 223, 59, 100, 90, 29}) != 123.123 {
		t.Fatal("Bytes_Float64() invalid bytes")
	}
	// wrong
	if Bytes_Int16([]byte{1}) != 0 {
		t.Fatal("Bytes_Int16() invalid bytes & result")
	}
	if Bytes_Int32([]byte{1}) != 0 {
		t.Fatal("Bytes_Int32() invalid bytes & result")
	}
	if Bytes_Int64([]byte{1}) != 0 {
		t.Fatal("Bytes_Int64() invalid bytes & result")
	}
	if Bytes_Uint16([]byte{1}) != 0 {
		t.Fatal("Bytes_Uint16() invalid bytes & result")
	}
	if Bytes_Uint32([]byte{1}) != 0 {
		t.Fatal("Bytes_Uint32() invalid bytes & result")
	}
	if Bytes_Uint64([]byte{1}) != 0 {
		t.Fatal("Bytes_Uint64() invalid bytes & result")
	}
	if Bytes_Float32([]byte{1}) != 0 {
		t.Fatal("Bytes_Float32() invalid bytes & result")
	}
	if Bytes_Float64([]byte{1}) != 0 {
		t.Fatal("Bytes_Float64() invalid bytes & result")
	}
	// -
	n := int64(-0x12345678)
	if Bytes_Int64(Int64_Bytes(n)) != n {
		t.Fatal("wrong n")
	}
}

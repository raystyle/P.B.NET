package convert

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

// Int16ToBytes is used to convert int16 to bytes
func Int16ToBytes(Int16 int16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(Int16))
	return b
}

// Int32ToBytes is used to convert int32 to bytes
func Int32ToBytes(Int32 int32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(Int32))
	return b
}

// Int64ToBytes is used to convert int64 to bytes
func Int64ToBytes(Int64 int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(Int64))
	return b
}

// Uint16ToBytes is used to convert uint16 to bytes
func Uint16ToBytes(Uint16 uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, Uint16)
	return b
}

// Uint32ToBytes is used to convert uint32 to bytes
func Uint32ToBytes(Uint32 uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, Uint32)
	return b
}

// Uint64ToBytes is used to convert uint64 to bytes
func Uint64ToBytes(Uint64 uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, Uint64)
	return b
}

// Float32ToBytes is used to convert float32 to bytes
func Float32ToBytes(Float32 float32) []byte {
	b := make([]byte, 4)
	n := *(*uint32)(unsafe.Pointer(&Float32))
	binary.BigEndian.PutUint32(b, n)
	return b
}

// Float64ToBytes is used to convert float64 to bytes
func Float64ToBytes(Float64 float64) []byte {
	b := make([]byte, 8)
	n := *(*uint64)(unsafe.Pointer(&Float64))
	binary.BigEndian.PutUint64(b, n)
	return b
}

// BytesToInt16 is used to convert bytes to int16
func BytesToInt16(Bytes []byte) int16 {
	if len(Bytes) != 2 {
		return 0
	}
	return int16(binary.BigEndian.Uint16(Bytes))
}

// BytesToInt32 is used to convert bytes to int32
func BytesToInt32(Bytes []byte) int32 {
	if len(Bytes) != 4 {
		return 0
	}
	return int32(binary.BigEndian.Uint32(Bytes))
}

// BytesToInt64 is used to convert bytes to int64
func BytesToInt64(Bytes []byte) int64 {
	if len(Bytes) != 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(Bytes))
}

// BytesToUint16 is used to convert bytes to uint16
func BytesToUint16(Bytes []byte) uint16 {
	if len(Bytes) != 2 {
		return 0
	}
	return binary.BigEndian.Uint16(Bytes)
}

// BytesToUint32 is used to convert bytes to uint32
func BytesToUint32(Bytes []byte) uint32 {
	if len(Bytes) != 4 {
		return 0
	}
	return binary.BigEndian.Uint32(Bytes)
}

// BytesToUint64 is used to convert bytes to uint64
func BytesToUint64(Bytes []byte) uint64 {
	if len(Bytes) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(Bytes)
}

// BytesToFloat32 is used to convert bytes to float32
func BytesToFloat32(Bytes []byte) float32 {
	if len(Bytes) != 4 {
		return 0
	}
	b := binary.BigEndian.Uint32(Bytes)
	return *(*float32)(unsafe.Pointer(&b))
}

// BytesToFloat64 is used to convert bytes to float64
func BytesToFloat64(Bytes []byte) float64 {
	if len(Bytes) != 8 {
		return 0
	}
	b := binary.BigEndian.Uint64(Bytes)
	return *(*float64)(unsafe.Pointer(&b))
}

// ByteToString is used to covert Byte to KB, MB, GB or TB
func ByteToString(n uint64) string {
	const (
		kb = 1 << 10
		mb = 1 << 20
		gb = 1 << 30
		tb = 1 << 40
	)
	switch {
	case n < kb:
		return fmt.Sprintf("%d Byte", n)
	case n < mb:
		return fmt.Sprintf("%.3f KB", float64(n)/kb)
	case n < gb:
		return fmt.Sprintf("%.3f MB", float64(n)/mb)
	case n < tb:
		return fmt.Sprintf("%.3f GB", float64(n)/gb)
	default:
		return fmt.Sprintf("%.3f TB", float64(n)/tb)
	}
}

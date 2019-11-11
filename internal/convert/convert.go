package convert

import (
	"encoding/binary"
	"unsafe"
)

func Int16ToBytes(Int16 int16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(Int16))
	return b
}

func Int32ToBytes(Int32 int32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(Int32))
	return b
}

func Int64ToBytes(Int64 int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(Int64))
	return b
}

func Uint16ToBytes(Uint16 uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, Uint16)
	return b
}

func Uint32ToBytes(Uint32 uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, Uint32)
	return b
}

func Uint64ToBytes(Uint64 uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, Uint64)
	return b
}

func Float32ToBytes(Float32 float32) []byte {
	b := make([]byte, 4)
	n := *(*uint32)(unsafe.Pointer(&Float32))
	binary.BigEndian.PutUint32(b, n)
	return b
}

func Float64ToBytes(Float64 float64) []byte {
	b := make([]byte, 8)
	n := *(*uint64)(unsafe.Pointer(&Float64))
	binary.BigEndian.PutUint64(b, n)
	return b
}

func BytesToInt16(Bytes []byte) int16 {
	if len(Bytes) != 2 {
		return 0
	}
	return int16(binary.BigEndian.Uint16(Bytes))
}

func BytesToInt32(Bytes []byte) int32 {
	if len(Bytes) != 4 {
		return 0
	}
	return int32(binary.BigEndian.Uint32(Bytes))
}

func BytesToInt64(Bytes []byte) int64 {
	if len(Bytes) != 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(Bytes))
}

func BytesToUint16(Bytes []byte) uint16 {
	if len(Bytes) != 2 {
		return 0
	}
	return binary.BigEndian.Uint16(Bytes)
}

func BytesToUint32(Bytes []byte) uint32 {
	if len(Bytes) != 4 {
		return 0
	}
	return binary.BigEndian.Uint32(Bytes)
}

func BytesToUint64(Bytes []byte) uint64 {
	if len(Bytes) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(Bytes)
}

func BytesToFloat32(Bytes []byte) float32 {
	if len(Bytes) != 4 {
		return 0
	}
	b := binary.BigEndian.Uint32(Bytes)
	return *(*float32)(unsafe.Pointer(&b))
}

func BytesToFloat64(Bytes []byte) float64 {
	if len(Bytes) != 8 {
		return 0
	}
	b := binary.BigEndian.Uint64(Bytes)
	return *(*float64)(unsafe.Pointer(&b))
}

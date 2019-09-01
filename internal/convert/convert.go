package convert

import (
	"encoding/binary"
	"math"
)

func Int16ToBytes(Int16 int16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, uint16(Int16))
	return buffer
}

func Int32ToBytes(Int32 int32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, uint32(Int32))
	return buffer
}

func Int64ToBytes(Int64 int64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, uint64(Int64))
	return buffer
}

func Uint16ToBytes(Uint16 uint16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, Uint16)
	return buffer
}

func Uint32ToBytes(Uint32 uint32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, Uint32)
	return buffer
}

func Uint64ToBytes(Uint64 uint64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, Uint64)
	return buffer
}

func Float32ToBytes(Float32 float32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, math.Float32bits(Float32))
	return buffer
}

func Float64ToBytes(Float64 float64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, math.Float64bits(Float64))
	return buffer
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
	return math.Float32frombits(binary.BigEndian.Uint32(Bytes))
}

func BytesToFloat64(Bytes []byte) float64 {
	if len(Bytes) != 8 {
		return 0
	}
	return math.Float64frombits(binary.BigEndian.Uint64(Bytes))
}

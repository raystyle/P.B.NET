package convert

import (
	"encoding/binary"
	"math"
)

func Int16_Bytes(Int16 int16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, uint16(Int16))
	return buffer
}

func Int32_Bytes(Int32 int32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, uint32(Int32))
	return buffer
}

func Int64_Bytes(Int64 int64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, uint64(Int64))
	return buffer
}

func Uint16_Bytes(Uint16 uint16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, Uint16)
	return buffer
}

func Uint32_Bytes(Uint32 uint32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, Uint32)
	return buffer
}

func Uint64_Bytes(Uint64 uint64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, Uint64)
	return buffer
}

func Float32_Bytes(Float32 float32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, math.Float32bits(Float32))
	return buffer
}

func Float64_Bytes(Float64 float64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, math.Float64bits(Float64))
	return buffer
}

func Bytes_Int16(Bytes []byte) int16 {
	if len(Bytes) != 2 {
		return 0
	}
	return int16(binary.BigEndian.Uint16(Bytes))
}

func Bytes_Int32(Bytes []byte) int32 {
	if len(Bytes) != 4 {
		return 0
	}
	return int32(binary.BigEndian.Uint32(Bytes))
}

func Bytes_Int64(Bytes []byte) int64 {
	if len(Bytes) != 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(Bytes))
}

func Bytes_Uint16(Bytes []byte) uint16 {
	if len(Bytes) != 2 {
		return 0
	}
	return binary.BigEndian.Uint16(Bytes)
}

func Bytes_Uint32(Bytes []byte) uint32 {
	if len(Bytes) != 4 {
		return 0
	}
	return binary.BigEndian.Uint32(Bytes)
}

func Bytes_Uint64(Bytes []byte) uint64 {
	if len(Bytes) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(Bytes)
}

func Bytes_Float32(Bytes []byte) float32 {
	if len(Bytes) != 4 {
		return 0
	}
	return math.Float32frombits(binary.BigEndian.Uint32(Bytes))
}

func Bytes_Float64(Bytes []byte) float64 {
	if len(Bytes) != 8 {
		return 0
	}
	return math.Float64frombits(binary.BigEndian.Uint64(Bytes))
}

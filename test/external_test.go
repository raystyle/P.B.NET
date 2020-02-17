package test

import (
	"bytes"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/guid"
)

func TestToml_Negative(t *testing.T) {
	cfg := struct {
		Num uint
	}{}
	err := toml.Unmarshal([]byte(`Num = -1`), &cfg)
	require.Error(t, err)
	require.Equal(t, uint(0), cfg.Num)
}

// fast
func BenchmarkBytes_Compare_GUID(b *testing.B) {
	benchmarkBytesCompare(b, guid.Size)
}

// slow
func BenchmarkBytes_Equal_GUID(b *testing.B) {
	benchmarkBytesEqual(b, guid.Size)
}

// slow
func BenchmarkBytes_Compare_1Bytes(b *testing.B) {
	benchmarkBytesCompare(b, 1)
}

// fast
func BenchmarkBytes_Equal_1Bytes(b *testing.B) {
	benchmarkBytesEqual(b, 1)
}

// slow
func BenchmarkBytes_Compare_4Bytes(b *testing.B) {
	benchmarkBytesCompare(b, 4)
}

// fast
func BenchmarkBytes_Equal_4Bytes(b *testing.B) {
	benchmarkBytesEqual(b, 4)
}

// slow
func BenchmarkBytes_Compare_8Bytes(b *testing.B) {
	benchmarkBytesCompare(b, 8)
}

// fast
func BenchmarkBytes_Equal_8Bytes(b *testing.B) {
	benchmarkBytesEqual(b, 8)
}

// slow
func BenchmarkBytes_Compare_10xGUID(b *testing.B) {
	benchmarkBytesCompare(b, 10*guid.Size)
}

// fast
func BenchmarkBytes_Equal_10xGUID(b *testing.B) {
	benchmarkBytesEqual(b, 10*guid.Size)
}

func benchmarkBytesCompare(b *testing.B, size int) {
	aa := bytes.Repeat([]byte{0}, size)
	bb := bytes.Repeat([]byte{0}, size)
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bytes.Compare(aa, bb)
	}
	b.StopTimer()
}

func benchmarkBytesEqual(b *testing.B, size int) {
	aa := bytes.Repeat([]byte{0}, size)
	bb := bytes.Repeat([]byte{0}, size)
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bytes.Equal(aa, bb)
	}
	b.StopTimer()
}

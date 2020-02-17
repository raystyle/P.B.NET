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
	aa := bytes.Repeat([]byte{0}, guid.Size)
	bb := bytes.Repeat([]byte{0}, guid.Size)
	benchmarkBytesCompare(b, aa, bb)
}

// slow
func BenchmarkBytes_Equal_GUID(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, guid.Size)
	bb := bytes.Repeat([]byte{0}, guid.Size)
	benchmarkBytesEqual(b, aa, bb)
}

// slow
func BenchmarkBytes_Compare_1Bytes(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 1)
	bb := bytes.Repeat([]byte{0}, 1)
	benchmarkBytesCompare(b, aa, bb)
}

// fast
func BenchmarkBytes_Equal_1Bytes(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 1)
	bb := bytes.Repeat([]byte{0}, 1)
	benchmarkBytesEqual(b, aa, bb)
}

// slow
func BenchmarkBytes_Compare_4Bytes(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 4)
	bb := bytes.Repeat([]byte{0}, 4)
	benchmarkBytesCompare(b, aa, bb)
}

// fast
func BenchmarkBytes_Equal_4Bytes(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 4)
	bb := bytes.Repeat([]byte{0}, 4)
	benchmarkBytesEqual(b, aa, bb)
}

// slow
func BenchmarkBytes_Compare_8Bytes(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 8)
	bb := bytes.Repeat([]byte{0}, 8)
	benchmarkBytesCompare(b, aa, bb)
}

// fast
func BenchmarkBytes_Equal_8Bytes(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 8)
	bb := bytes.Repeat([]byte{0}, 8)
	benchmarkBytesEqual(b, aa, bb)
}

// slow
func BenchmarkBytes_Compare_10xGUID(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 10*guid.Size)
	bb := bytes.Repeat([]byte{0}, 10*guid.Size)
	benchmarkBytesCompare(b, aa, bb)
}

// fast
func BenchmarkBytes_Equal_10xGUID(b *testing.B) {
	aa := bytes.Repeat([]byte{0}, 10*guid.Size)
	bb := bytes.Repeat([]byte{0}, 10*guid.Size)
	benchmarkBytesEqual(b, aa, bb)
}

func benchmarkBytesCompare(b *testing.B, aa, bb []byte) {
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bytes.Compare(aa, bb)
	}
	b.StopTimer()
}

func benchmarkBytesEqual(b *testing.B, aa, bb []byte) {
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		bytes.Equal(aa, bb)
	}
	b.StopTimer()
}

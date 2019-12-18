package random

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandom(t *testing.T) {
	s := String(10)
	require.True(t, len(s) == 10)
	t.Log(s)
	b := Bytes(10)
	require.True(t, len(b) == 10)
	t.Log(b)
	c := Cookie(10)
	require.True(t, len(c) == 10)
	t.Log(c)
	i := Int(10)
	require.True(t, i >= 0 && i < 10)
	t.Log(i)
	t.Log(Int64())
	t.Log(Uint64())
	Sleep(1, 2)
	// for select
	for i := 0; i < 10240; i++ {
		Cookie(32)
	}
	// < 1
	require.True(t, len(String(-1)) == 0)
	require.True(t, len(Bytes(-1)) == 0)
	require.True(t, len(Cookie(-1)) == 0)
	require.True(t, Int(-1) == 0)
}

func TestRandomEqual(t *testing.T) {
	const n = 256
	result := make(chan int, n)
	for i := 0; i < n; i++ {
		go func() {
			r := New()
			result <- r.Int(math.MaxInt64)
		}()
	}
	results := make(map[int]*struct{})
	for i := 0; i < n; i++ {
		r := <-result
		_, ok := results[r]
		require.False(t, ok, "appeared value: %d, i: %d", r, i)
		results[r] = new(struct{})
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		New()
	}
}

func BenchmarkRand_Bytes(b *testing.B) {
	rand := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rand.Bytes(16)
	}
}

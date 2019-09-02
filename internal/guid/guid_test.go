package guid

import (
	"testing"
	"time"
)

func TestGUID(t *testing.T) {
	g := New(16, nil)
	for i := 0; i < 4; i++ {
		t.Log(g.Get())
	}
	g.Close()
	// now
	g = New(16, time.Now)
	for i := 0; i < 4; i++ {
		t.Log(g.Get())
	}
	g.Close()
	// 0 size
	g = New(-1, time.Now)
	for i := 0; i < 4; i++ {
		t.Log(g.Get())
	}
	g.Close()
	// twice
	g.Close()
}

func BenchmarkGenerator_Get(b *testing.B) {
	g := New(512, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Get()
	}
	b.StopTimer()
}

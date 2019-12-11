package guid

import (
	"testing"
	"time"

	"project/internal/testsuite"
)

func TestGenerator(t *testing.T) {
	g := New(16, nil)
	for i := 0; i < 4; i++ {
		t.Log(g.Get())
	}
	g.Close()
	testsuite.IsDestroyed(t, g)
	// with now
	g = New(16, time.Now)
	for i := 0; i < 4; i++ {
		t.Log(g.Get())
	}
	g.Close()
	testsuite.IsDestroyed(t, g)
	// 0 size
	g = New(0, time.Now)
	for i := 0; i < 4; i++ {
		t.Log(g.Get())
	}
	g.Close()
	// twice
	g.Close()
	testsuite.IsDestroyed(t, g)
}

func BenchmarkGenerator_Get(b *testing.B) {
	g := New(512, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Get()
	}
	b.StopTimer()
	g.Close()
	testsuite.IsDestroyed(b, g)
}

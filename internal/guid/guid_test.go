package guid

import (
	"testing"
	"time"
)

func Test_GUID(t *testing.T) {
	guid_generator := New(16, nil)
	for i := 0; i < 4; i++ {
		t.Log(guid_generator.Get())
	}
	guid_generator.Close()
	//now
	guid_generator = New(16, time.Now)
	for i := 0; i < 4; i++ {
		t.Log(guid_generator.Get())
	}
	guid_generator.Close()
	//0 size
	guid_generator = New(-1, time.Now)
	for i := 0; i < 4; i++ {
		t.Log(guid_generator.Get())
	}
	guid_generator.Close()
	// twice
	guid_generator.Close()
}

func Benchmark_Get(b *testing.B) {
	g := New(512, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Get()
	}
	b.StopTimer()
}

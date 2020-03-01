package testsuite

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// GoroutineMark contains testing.TB and then goroutine number.
type GoroutineMark struct {
	t    testing.TB
	then int
}

// MarkGoroutines is used to mark the number of the goroutines.
func MarkGoroutines(t testing.TB) *GoroutineMark {
	return &GoroutineMark{
		t:    t,
		then: runtime.NumGoroutine(),
	}
}

func (m *GoroutineMark) calculate() int {
	// total 3 seconds
	var n int
	for i := 0; i < 300; i++ {
		n = runtime.NumGoroutine() - m.then
		if n == 0 {
			return 0
		}
		time.Sleep(10 * time.Millisecond)
	}
	return runtime.NumGoroutine() - m.then
}

// Compare is used to compare the number of the goroutines.
func (m *GoroutineMark) Compare() {
	const format = "goroutine leaks! then: %d now: %d"
	now := runtime.NumGoroutine()
	require.Equalf(m.t, 0, m.calculate(), format, m.then, now)
}

// MemoryMark contains testing.TB, then and now memory status.
type MemoryMark struct {
	t    testing.TB
	then *runtime.MemStats
	now  *runtime.MemStats
}

// MarkMemory is used to mark the memory status.
func MarkMemory(t testing.TB) *MemoryMark {
	m := &MemoryMark{
		t:    t,
		then: new(runtime.MemStats),
		now:  new(runtime.MemStats),
	}
	runtime.GC()
	runtime.ReadMemStats(m.then)
	return m
}

func (m *MemoryMark) calculate() bool {
	runtime.GC()
	runtime.ReadMemStats(m.now)
	return true
}

// Compare is used to compare the memory status.
func (m *MemoryMark) Compare() {
	require.True(m.t, m.calculate(), "memory leaks")
}

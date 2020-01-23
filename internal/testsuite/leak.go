package testsuite

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// GoroutineMark contains testing.TB and then goroutine number
type GoroutineMark struct {
	t    testing.TB
	then int
}

// MarkGoroutines is used to mark the number of the goroutines
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

// Compare is used to compare the number of the goroutines
func (m *GoroutineMark) Compare() {
	require.Equal(m.t, 0, m.calculate(), "goroutine leaks")
}

// MemoryMark contains testing.TB, then and now memory status
type MemoryMark struct {
	t    testing.TB
	then *runtime.MemStats
	now  *runtime.MemStats
}

// MarkMemory is used to mark the memory status
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

	then := reflect.ValueOf(*m.then)
	thenType := reflect.TypeOf(*m.then)
	now := reflect.ValueOf(*m.now)
	for i := 0; i < then.NumField(); i++ {
		name := thenType.Field(i).Name
		f1 := then.Field(i)
		f2 := now.Field(i)
		fmt.Println(name, f2.Uint(), f1.Uint(), f2.Uint()-f1.Uint())
		if i == 23 {
			break
		}
	}
	return m.now.HeapObjects < m.then.HeapObjects
}

// Compare is used to compare the memory status
func (m *MemoryMark) Compare() {
	require.True(m.t, m.calculate(), "memory leaks")
}

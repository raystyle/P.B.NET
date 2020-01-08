package testsuite

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// GoRoutineMark contains testing.TB and then go routines number
type GoRoutineMark struct {
	t    testing.TB
	then int
}

// MarkGoRoutines is used to mark the number of the go routines
func MarkGoRoutines(t testing.TB) *GoRoutineMark {
	return &GoRoutineMark{
		t:    t,
		then: runtime.NumGoroutine(),
	}
}

func (m *GoRoutineMark) calculate() int {
	// total 3 second
	var n int
	for i := 0; i < 60; i++ {
		n = runtime.NumGoroutine() - m.then
		if n == 0 {
			return 0
		}
		time.Sleep(50 * time.Millisecond)
	}
	return runtime.NumGoroutine() - m.then
}

// Compare is used to compare the number of the go routines
func (m *GoRoutineMark) Compare() {
	require.Equal(m.t, 0, m.calculate(), "go routine leaks")
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

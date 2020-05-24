package testsuite

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/require"
)

// GoroutineMark contains testing.TB and then goroutine number.
type GoroutineMark struct {
	t testing.TB

	// the number of the goroutine
	then int

	// goroutine stack record
	record *bytes.Buffer

	// the number of the goroutine
	now int
}

// MarkGoroutines is used to mark the number of the goroutines.
func MarkGoroutines(t testing.TB) *GoroutineMark {
	// save current goroutine stack record
	num := runtime.NumGoroutine()
	buf := bytes.NewBuffer(make([]byte, 0, 1024*num))
	profile := pprof.Lookup("goroutine")
	err := profile.WriteTo(buf, 1)
	require.NoError(t, err)
	return &GoroutineMark{
		t:      t,
		then:   num,
		record: buf,
	}
}

// total wait 3 seconds for wait goroutine return.
func (m *GoroutineMark) calculate() int {
	for i := 0; i < 300; i++ {
		if runtime.NumGoroutine()-m.then == 0 {
			return 0
		}
		time.Sleep(10 * time.Millisecond)
	}
	// print then goroutine stack record
	fmt.Println("---------then goroutine stack record----------")
	fmt.Println(m.record)
	// print current goroutine stack record
	num := runtime.NumGoroutine()
	buf := bytes.NewBuffer(make([]byte, 0, 1024*num))
	profile := pprof.Lookup("goroutine")
	err := profile.WriteTo(buf, 1)
	require.NoError(m.t, err)
	fmt.Println("--------current goroutine stack record--------")
	fmt.Println(buf)
	// print different
	fmt.Println("--------different between then and now--------")
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(m.record.String()),
		B:        difflib.SplitLines(buf.String()),
		FromFile: "Expected",
		FromDate: "",
		ToFile:   "Actual",
		ToDate:   "",
		Context:  1,
	})
	fmt.Println(diff)
	// save current goroutine number
	m.now = num
	return m.now - m.then
}

// Compare is used to compare the number of the goroutines.
func (m *GoroutineMark) Compare() {
	const format = "goroutine leaks! then: %d now: %d"
	delta := m.calculate()
	require.Equalf(m.t, 0, delta, format, m.then, m.now)
}

// Destroyed is used to check if the object has been recycled by the GC.
// It not need testing.TB.
func Destroyed(object interface{}) bool {
	destroyed := make(chan struct{})
	runtime.SetFinalizer(object, func(interface{}) {
		close(destroyed)
	})
	// total 3 seconds
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	for i := 0; i < 300; i++ {
		timer.Reset(10 * time.Millisecond)
		runtime.GC()
		select {
		case <-destroyed:
			return true
		case <-timer.C:
		}
	}
	return false
}

// IsDestroyed is used to check if the object has been recycled by the GC.
func IsDestroyed(t testing.TB, object interface{}) {
	require.True(t, Destroyed(object), "object not destroyed")
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

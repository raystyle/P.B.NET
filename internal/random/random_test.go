package random

import (
	"crypto/sha256"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

// copy from testsuite
func testDeferForPanic(t testing.TB) {
	r := recover()
	require.NotNil(t, r)
	t.Logf("\npanic in %s:\n%s\n", t.Name(), r)
}

func TestRandom(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		str := String(10)
		require.Len(t, str, 10)
		t.Log(str)

		str = String(-1)
		require.Len(t, str, 0)
	})

	t.Run("Bytes", func(t *testing.T) {
		bytes := Bytes(10)
		require.Len(t, bytes, 10)
		t.Log(bytes)

		bytes = Bytes(-1)
		require.Len(t, bytes, 0)
	})

	t.Run("Cookie", func(t *testing.T) {
		cookie := Cookie(10)
		require.Len(t, cookie, 10)
		t.Log(cookie)

		cookie = Cookie(-1)
		require.Len(t, cookie, 0)
	})

	t.Run("Cookie-collide", func(t *testing.T) {
		for i := 0; i < 10240; i++ {
			Cookie(32)
		}
	})

	t.Run("Int", func(t *testing.T) {
		i := Int(10)
		require.True(t, i >= 0 && i < 10)
		t.Log(i)

		t.Log(Int64())
		t.Log(Uint64())

		require.True(t, Int(-1) == 0)
	})

	t.Run("panic about rand.New 1", func(t *testing.T) {
		defer testDeferForPanic(t)

		patch := func(rand.Source) *rand.Rand {
			panic(monkey.Panic)
		}
		pg := monkey.Patch(rand.New, patch)
		defer pg.Unpatch()

		NewRand()
	})

	t.Run("panic about rand.New 2", func(t *testing.T) {
		defer func() { time.Sleep(2 * time.Second) }()
		defer testDeferForPanic(t)

		hash := sha256.New()
		patch := func(interface{}, []byte) (int, error) {
			panic(monkey.Panic)
		}
		pg := monkey.PatchInstanceMethod(hash, "Write", patch)
		defer pg.Unpatch()

		NewRand()
	})
}

func TestRandomEqual(t *testing.T) {
	const n = 64
	result := make(chan int, n)
	for i := 0; i < n; i++ {
		go func() {
			r := NewRand()
			result <- r.Int(1048576)
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
		NewRand()
	}
}

func BenchmarkRand_Bytes(b *testing.B) {
	r := NewRand()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Bytes(16)
	}
}

func TestSleeper(t *testing.T) {
	// wait global sleeper
	time.Sleep(100 * time.Millisecond)

	t.Run("common", func(t *testing.T) {
		<-Sleep(1, 2)
	})

	t.Run("zero", func(t *testing.T) {
		<-Sleep(0, 0)
	})

	t.Run("not read", func(t *testing.T) {
		Sleep(0, 0)
		time.Sleep(time.Second + 100*time.Millisecond)
		Sleep(0, 0)
	})

	t.Run("max", func(t *testing.T) {
		d := gSleeper.calculateDuration(3600, 3600)
		require.Equal(t, MaxSleepTime, d)
	})

	gSleeper.Stop()
}

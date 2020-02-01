package guid

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/random"
	"project/internal/testsuite"
)

func TestGUID(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		guid := GUID{}
		copy(guid[Size/2:], bytes.Repeat([]byte{1}, Size/2))
		buf := bytes.Buffer{}
		buf.WriteString("GUID: ")
		buf.WriteString(strings.Repeat("00", Size/2))
		buf.WriteString("\n      ")
		buf.WriteString(strings.Repeat("01", Size/2))
		require.Equal(t, buf.String(), guid.String())
	})

	t.Run("Hex", func(t *testing.T) {
		guid := GUID{}
		copy(guid[Size/2:], bytes.Repeat([]byte{1}, Size/2))
		buf := bytes.Buffer{}
		buf.WriteString(strings.Repeat("00", Size/2))
		buf.WriteString("\n")
		buf.WriteString(strings.Repeat("01", Size/2))
		require.Equal(t, buf.String(), guid.Hex())
	})

	t.Run("Timestamp", func(t *testing.T) {
		now := time.Now().Unix()
		guid := GUID{}
		copy(guid[32:40], convert.Int64ToBytes(now))
		require.Equal(t, now, guid.Timestamp())
	})
}

func TestGenerator(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("with no now function", func(t *testing.T) {
		g := New(16, nil)
		for i := 0; i < 4; i++ {
			guid := g.Get()[:]
			t.Log(guid)
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("with now()", func(t *testing.T) {
		g := New(16, time.Now)
		for i := 0; i < 4; i++ {
			guid := g.Get()[:]
			t.Log(guid)
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("zero size", func(t *testing.T) {
		g := New(0, time.Now)
		for i := 0; i < 4; i++ {
			t.Log(g.Get()[:])
		}
		g.Close()
		// twice
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("panic in generate()", func(t *testing.T) {
		patchFunc := func(uint64) []byte {
			panic(testsuite.ErrMonkey)
		}
		pg := testsuite.Patch(convert.Uint64ToBytes, patchFunc)
		go func() {
			time.Sleep(time.Second)
			pg.Unpatch()
		}()
		g := New(0, time.Now)
		for i := 0; i < 4; i++ {
			t.Log(g.Get()[:])
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})
}

func BenchmarkGenerator_Get(b *testing.B) {
	gm := testsuite.MarkGoroutines(b)
	defer gm.Compare()

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

func BenchmarkGUIDWithMapKey(b *testing.B) {
	gm := testsuite.MarkGoroutines(b)
	defer gm.Compare()

	rand := random.New()
	key := make([]GUID, b.N)
	for i := 0; i < b.N; i++ {
		b := rand.Bytes(Size)
		copy(key[i][:], b)
	}
	m := make(map[GUID]int)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[key[i]] = i
	}
	b.StopTimer()
}

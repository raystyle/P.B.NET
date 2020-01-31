package guid

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestGenerator(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		g := New(16, nil)
		for i := 0; i < 4; i++ {
			guid := g.Get()
			require.Equal(t, Size, len(guid))
			t.Log(guid)
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("with now()", func(t *testing.T) {
		g := New(16, time.Now)
		for i := 0; i < 4; i++ {
			guid := g.Get()
			require.Equal(t, Size, len(guid))
			t.Log(guid)
		}
		g.Close()
		testsuite.IsDestroyed(t, g)
	})

	t.Run("zero size", func(t *testing.T) {
		g := New(0, time.Now)
		for i := 0; i < 4; i++ {
			t.Log(g.Get())
		}
		g.Close()
		// twice
		g.Close()
		testsuite.IsDestroyed(t, g)
	})
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

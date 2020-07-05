package security

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoverBytes(t *testing.T) {
	b1 := []byte{1, 2, 3, 4}
	b2 := []byte{1, 2, 3, 4}
	CoverBytes(b2)
	require.NotEqual(t, b1, b2, "failed to cover bytes")
}

func TestCoverString(t *testing.T) {
	// must use strings.Repeat to generate testdata
	// if you use this
	// s1 := "aaa"
	// s2 := "aaa"
	// CoverString(&s1) will panic, because it change const.

	s1 := strings.Repeat("a", 10)
	s2 := strings.Repeat("a", 10)
	CoverString(s2)
	require.NotEqual(t, s1, s2, "failed to cover string")
}

func TestCoverStringMap(t *testing.T) {
	s1 := strings.Repeat("a", 10)
	s2 := strings.Repeat("a", 10)

	m := map[string]struct{}{
		s1: {},
	}

	CoverStringMap(m)

	var str string
	for str = range m {
	}

	require.NotEqual(t, str, s2, "failed to cover string map")
}

func TestBytes(t *testing.T) {
	testdata := []byte{1, 2, 3, 4}

	sb := NewBytes(testdata)

	t.Run("common", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			b := sb.Get()
			require.Equal(t, testdata, b)
			sb.Put(b)
		}
	})

	t.Run("parallel", func(t *testing.T) {
		wg := sync.WaitGroup{}
		wg.Add(100)
		for i := 0; i < 100; i++ {
			go func() {
				defer wg.Done()
				for i := 0; i < 10; i++ {
					b := sb.Get()
					require.Equal(t, testdata, b)
					sb.Put(b)
				}
			}()
		}
		wg.Wait()
	})
}

func TestBogoWait(t *testing.T) {
	t.Run("common", func(t *testing.T) {
		BogoWait(4, time.Minute)
	})

	t.Run("timeout", func(t *testing.T) {
		BogoWait(1024, time.Second)
	})

	t.Run("invalid n or timeout", func(t *testing.T) {
		BogoWait(0, time.Hour)
	})
}

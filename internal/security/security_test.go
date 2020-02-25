package security

import (
	"bytes"
	"strings"
	"sync"
	"testing"

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
	// CoverString(&s1) will panic, because it change const
	s1 := strings.Repeat("a", 10)
	s2 := strings.Repeat("a", 10)
	CoverString(&s2)
	require.NotEqual(t, s1, s2, "failed to cover string")
}

func TestBytes(t *testing.T) {
	testdata := []byte{1, 2, 3, 4}
	sb := NewBytes(testdata)
	for i := 0; i < 10; i++ {
		b := sb.Get()
		require.True(t, bytes.Equal(testdata, b))
		sb.Put(b)
	}
	wg := sync.WaitGroup{}
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				b := sb.Get()
				require.True(t, bytes.Equal(testdata, b))
				sb.Put(b)
			}
		}()
	}
	wg.Wait()
}

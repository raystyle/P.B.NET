package security

import (
	"bytes"
	"net/http"
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
	// CoverString(&s1) will panic, because it change const
	s1 := strings.Repeat("a", 10)
	s2 := strings.Repeat("a", 10)
	CoverString(&s2)
	require.NotEqual(t, s1, s2, "failed to cover string")
}

func TestCoverHTTPRequest(t *testing.T) {
	url := strings.Repeat("http://test.com/", 1)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	f1 := req.URL.String()
	CoverHTTPRequest(req)
	f2 := req.URL.String()
	require.NotEqual(t, f1, f2, "failed to cover string fields")
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
			time.Sleep(10 * time.Millisecond)
			for i := 0; i < 10; i++ {
				b := sb.Get()
				require.True(t, bytes.Equal(testdata, b))
				sb.Put(b)
			}
		}()
	}
	wg.Wait()
}

package certutil

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemCertPool(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			pool, err := SystemCertPool()
			require.NoError(t, err)
			t.Log("the number of the system certificates:", len(pool.Subjects()))

			for _, sub := range pool.Subjects() {
				t.Log(string(sub))
			}
		}()
	}
	wg.Wait()
}

package certpool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystem(t *testing.T) {
	wg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			pool, err := System()
			require.NoError(t, err)
			num := len(pool.Subjects())
			t.Log("the number of the system certificates:", num)

			for _, subject := range pool.Subjects() {
				t.Log(string(subject))
			}
		}()
	}
	wg.Wait()
}

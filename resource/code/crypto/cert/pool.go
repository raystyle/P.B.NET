package cert

import (
	"fmt"
	"strings"
)

func generateTestPoolAddCertParallel(f1 string) {
	const template = `
func TestPool_Add<f1>Cert_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pair1 := testGeneratePair(t)
	pair2 := testGeneratePair(t)

	t.Run("part", func(t *testing.T) {
		pool := NewPool()

		add1 := func() {
			err := pool.Add<f1>Cert(pair1.Certificate.Raw)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add<f1>Cert(pair2.Certificate.Raw)
			require.NoError(t, err)
		}
		add3 := func() {
			err := pool.Add<f1>Cert(nil)
			require.Error(t, err)
		}
		cleanup := func() {
			certs := pool.Get<f1>Certs()
			require.Len(t, certs, 2)

			err := pool.Delete<f1>Cert(0)
			require.NoError(t, err)
			err = pool.Delete<f1>Cert(0)
			require.NoError(t, err)

			certs = pool.Get<f1>Certs()
			require.Len(t, certs, 0)
		}
		testsuite.RunParallel(100, nil, cleanup, add1, add2, add3)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("whole", func(t *testing.T) {
		var pool *Pool

		init := func() {
			pool = NewPool()
		}
		add1 := func() {
			err := pool.Add<f1>Cert(pair1.Certificate.Raw)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add<f1>Cert(pair2.Certificate.Raw)
			require.NoError(t, err)
		}
		add3 := func() {
			err := pool.Add<f1>Cert(nil)
			require.Error(t, err)
		}
		cleanup := func() {
			certs := pool.Get<f1>Certs()
			require.Len(t, certs, 2)
		}
		testsuite.RunParallel(100, init, cleanup, add1, add2, add3)

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pair1)
	testsuite.IsDestroyed(t, pair2)
}
`
	code := strings.ReplaceAll(template, "<f1>", f1)
	fmt.Println(code)
}

func generateTestPoolAddPairParallel(f1 string) {
	const template = `
func TestPool_Add<f1>Pair_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pair1 := testGeneratePair(t)
	pair2 := testGeneratePair(t)
	cert1, key1 := pair1.Encode()
	cert2, key2 := pair2.Encode()

	t.Run("part", func(t *testing.T) {
		pool := NewPool()

		add1 := func() {
			err := pool.Add<f1>Pair(cert1, key1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add<f1>Pair(cert2, key2)
			require.NoError(t, err)
		}
		add3 := func() {
			err := pool.Add<f1>Pair(nil, nil)
			require.Error(t, err)
		}
		cleanup := func() {
			pairs := pool.Get<f1>Pairs()
			require.Len(t, pairs, 2)

			err := pool.Delete<f1>Cert(0)
			require.NoError(t, err)
			err = pool.Delete<f1>Cert(0)
			require.NoError(t, err)

			pairs = pool.Get<f1>Pairs()
			require.Len(t, pairs, 0)
		}
		testsuite.RunParallel(100, nil, cleanup, add1, add2, add3)

		testsuite.IsDestroyed(t, pool)
	})

	t.Run("whole", func(t *testing.T) {
		var pool *Pool

		init := func() {
			pool = NewPool()
		}
		add1 := func() {
			err := pool.Add<f1>Pair(cert1, key1)
			require.NoError(t, err)
		}
		add2 := func() {
			err := pool.Add<f1>Pair(cert2, key2)
			require.NoError(t, err)
		}
		add3 := func() {
			err := pool.Add<f1>Pair(nil, nil)
			require.Error(t, err)
		}
		cleanup := func() {
			pairs := pool.Get<f1>Pairs()
			require.Len(t, pairs, 2)
		}
		testsuite.RunParallel(100, init, cleanup, add1, add2, add3)

		testsuite.IsDestroyed(t, pool)
	})

	testsuite.IsDestroyed(t, pair1)
	testsuite.IsDestroyed(t, pair2)
}
`
	code := strings.ReplaceAll(template, "<f1>", f1)
	fmt.Println(code)
}

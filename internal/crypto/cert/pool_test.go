package cert

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func testGenerateCert(t *testing.T) *Pair {
	pair, err := GenerateCA(nil)
	require.NoError(t, err)
	return pair
}

func testRunParallel(f ...func()) {
	l := len(f)
	if l == 0 {
		return
	}
	wg := sync.WaitGroup{}
	for i := 0; i < l; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f[i]()
		}(i)
	}
	wg.Wait()
}

func TestPool(t *testing.T) {
	pool := NewPool()

	t.Run("PublicRootCACert", func(t *testing.T) {
		require.Equal(t, 0, len(pool.GetPublicRootCACerts()))

		pair := testGenerateCert(t)

		t.Run("add", func(t *testing.T) {
			err := pool.AddPublicRootCACert(pair.Certificate)
			require.NoError(t, err)
			err = pool.AddPublicRootCACert(pair.Certificate)
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			certs := pool.GetPublicRootCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("parallel", func(t *testing.T) {
			pair := testGenerateCert(t)
			add := func() {
				err := pool.AddPublicRootCACert(pair.Certificate)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicRootCACerts()
			}
			testRunParallel(add, get)
		})
	})

	t.Run("PublicClientCACert", func(t *testing.T) {
		require.Equal(t, 0, len(pool.GetPublicClientCACerts()))

		pair := testGenerateCert(t)
		t.Run("add", func(t *testing.T) {
			err := pool.AddPublicClientCACert(pair.Certificate)
			require.NoError(t, err)
			err = pool.AddPublicClientCACert(pair.Certificate)
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			certs := pool.GetPublicClientCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("parallel", func(t *testing.T) {
			pair := testGenerateCert(t)
			add := func() {
				err := pool.AddPublicClientCACert(pair.Certificate)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicClientCACerts()
			}
			testRunParallel(add, get)
		})
	})

	t.Run("PublicClientCert", func(t *testing.T) {
		require.Equal(t, 0, len(pool.GetPublicClientPairs()))

		pair := testGenerateCert(t)
		t.Run("add", func(t *testing.T) {
			err := pool.AddPublicClientCert(pair.Certificate, pair.PrivateKey)
			require.NoError(t, err)
			err = pool.AddPublicClientCert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
			err = pool.AddPublicClientCert(pair.Certificate, nil)
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			pairs := pool.GetPublicClientPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("parallel", func(t *testing.T) {
			pair := testGenerateCert(t)
			add := func() {
				err := pool.AddPublicClientCert(pair.Certificate, pair.PrivateKey)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPublicClientPairs()
			}
			testRunParallel(add, get)
		})
	})

	t.Run("PrivateRootCACert", func(t *testing.T) {
		require.Equal(t, 0, len(pool.GetPrivateRootCACerts()))

		pair := testGenerateCert(t)
		t.Run("add", func(t *testing.T) {
			err := pool.AddPrivateRootCACert(pair.Certificate, pair.PrivateKey)
			require.NoError(t, err)
			err = pool.AddPrivateRootCACert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
			err = pool.AddPrivateRootCACert(pair.Certificate, []byte{})
			require.Error(t, err)
		})

		t.Run("get certs", func(t *testing.T) {
			certs := pool.GetPrivateRootCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("get pairs", func(t *testing.T) {
			pairs := pool.GetPrivateRootCAPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("parallel", func(t *testing.T) {
			pair := testGenerateCert(t)
			add := func() {
				err := pool.AddPrivateRootCACert(pair.Certificate, pair.PrivateKey)
				require.NoError(t, err)
			}
			getCerts := func() {
				pool.GetPrivateRootCACerts()
			}
			getPairs := func() {
				pool.GetPrivateRootCAPairs()
			}
			testRunParallel(add, getCerts, getPairs)
		})
	})

	t.Run("PrivateClientCACert", func(t *testing.T) {
		require.Equal(t, 0, len(pool.GetPrivateClientCACerts()))

		pair := testGenerateCert(t)
		t.Run("add", func(t *testing.T) {
			err := pool.AddPrivateClientCACert(pair.Certificate, pair.PrivateKey)
			require.NoError(t, err)
			err = pool.AddPrivateClientCACert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
			err = pool.AddPrivateClientCACert(pair.Certificate, []byte{})
			require.Error(t, err)
		})

		t.Run("get certs", func(t *testing.T) {
			certs := pool.GetPrivateClientCACerts()
			require.True(t, certs[0].Equal(pair.Certificate))
		})

		t.Run("get pairs", func(t *testing.T) {
			pairs := pool.GetPrivateClientCAPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("parallel", func(t *testing.T) {
			pair := testGenerateCert(t)
			add := func() {
				err := pool.AddPrivateClientCACert(pair.Certificate, pair.PrivateKey)
				require.NoError(t, err)
			}
			getCerts := func() {
				pool.GetPrivateClientCACerts()
			}
			getPairs := func() {
				pool.GetPrivateClientCAPairs()
			}
			testRunParallel(add, getCerts, getPairs)
		})
	})

	t.Run("PrivateClientCert", func(t *testing.T) {
		require.Equal(t, 0, len(pool.GetPrivateClientPairs()))

		pair := testGenerateCert(t)
		t.Run("add", func(t *testing.T) {
			err := pool.AddPrivateClientCert(pair.Certificate, pair.PrivateKey)
			require.NoError(t, err)
			err = pool.AddPrivateClientCert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
			err = pool.AddPrivateClientCert(pair.Certificate, []byte{})
			require.Error(t, err)
		})

		t.Run("get", func(t *testing.T) {
			pairs := pool.GetPrivateClientPairs()
			require.Equal(t, pair, pairs[0])
		})

		t.Run("parallel", func(t *testing.T) {
			pair := testGenerateCert(t)
			add := func() {
				err := pool.AddPrivateClientCert(pair.Certificate, pair.PrivateKey)
				require.NoError(t, err)
			}
			get := func() {
				pool.GetPrivateClientPairs()
			}
			testRunParallel(add, get)
		})
	})
}

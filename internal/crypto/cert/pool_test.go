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

func testRunParallel(f func()) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
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
			testRunParallel(func() {
				certs := pool.GetPublicRootCACerts()
				require.True(t, certs[0].Equal(pair.Certificate))
			})
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
			testRunParallel(func() {
				certs := pool.GetPublicClientCACerts()
				require.True(t, certs[0].Equal(pair.Certificate))
			})
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
			testRunParallel(func() {
				pairs := pool.GetPublicClientPairs()
				require.Equal(t, pair, pairs[0])
			})
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
			testRunParallel(func() {
				certs := pool.GetPrivateRootCACerts()
				require.True(t, certs[0].Equal(pair.Certificate))
			})
		})

		t.Run("get pairs", func(t *testing.T) {
			testRunParallel(func() {
				pairs := pool.GetPrivateRootCAPairs()
				require.Equal(t, pair, pairs[0])
			})
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
			testRunParallel(func() {
				certs := pool.GetPrivateClientCACerts()
				require.True(t, certs[0].Equal(pair.Certificate))
			})
		})

		t.Run("get pairs", func(t *testing.T) {
			testRunParallel(func() {
				pairs := pool.GetPrivateClientCAPairs()
				require.Equal(t, pair, pairs[0])
			})
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
			testRunParallel(func() {
				pairs := pool.GetPrivateClientPairs()
				require.Equal(t, pair, pairs[0])
			})
		})
	})
}

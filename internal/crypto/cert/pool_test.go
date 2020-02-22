package cert

import (
	"crypto/x509"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/security"
)

func TestPair_ToPair(t *testing.T) {
	defer func() {
		require.NotNil(t, recover())
	}()
	pair := pair{
		PrivateKey: security.NewBytes(make([]byte, 1024)),
	}
	pair.ToPair()
}

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

		t.Run("invalid certificate", func(t *testing.T) {
			cert := new(x509.Certificate)
			cert.Raw = make([]byte, 1024)
			err := pool.AddPublicRootCACert(cert)
			require.Error(t, err)
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

		t.Run("invalid certificate", func(t *testing.T) {
			cert := new(x509.Certificate)
			cert.Raw = make([]byte, 1024)
			err := pool.AddPublicClientCACert(cert)
			require.Error(t, err)
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

		t.Run("invalid certificate", func(t *testing.T) {
			pair := testGenerateCert(t)
			pair.Certificate.Raw = make([]byte, 1024)
			err := pool.AddPublicClientCert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
		})

		t.Run("invalid private key", func(t *testing.T) {
			pair := testGenerateCert(t)
			patchFunc := func(_ interface{}) ([]byte, error) {
				return nil, monkey.ErrMonkey
			}
			pg := monkey.Patch(x509.MarshalPKCS8PrivateKey, patchFunc)
			defer pg.Unpatch()
			err := pool.AddPublicClientCert(pair.Certificate, pair.PrivateKey)
			monkey.IsMonkeyError(t, err)
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

		t.Run("invalid certificate", func(t *testing.T) {
			pair := testGenerateCert(t)
			pair.Certificate.Raw = make([]byte, 1024)
			err := pool.AddPrivateRootCACert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
		})

		t.Run("invalid private key", func(t *testing.T) {
			pair := testGenerateCert(t)
			patchFunc := func(_ interface{}) ([]byte, error) {
				return nil, monkey.ErrMonkey
			}
			pg := monkey.Patch(x509.MarshalPKCS8PrivateKey, patchFunc)
			defer pg.Unpatch()
			err := pool.AddPrivateRootCACert(pair.Certificate, pair.PrivateKey)
			monkey.IsMonkeyError(t, err)
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

		t.Run("invalid certificate", func(t *testing.T) {
			pair := testGenerateCert(t)
			pair.Certificate.Raw = make([]byte, 1024)
			err := pool.AddPrivateClientCACert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
		})

		t.Run("invalid private key", func(t *testing.T) {
			pair := testGenerateCert(t)
			patchFunc := func(_ interface{}) ([]byte, error) {
				return nil, monkey.ErrMonkey
			}
			pg := monkey.Patch(x509.MarshalPKCS8PrivateKey, patchFunc)
			defer pg.Unpatch()
			err := pool.AddPrivateClientCACert(pair.Certificate, pair.PrivateKey)
			monkey.IsMonkeyError(t, err)
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

		t.Run("invalid certificate", func(t *testing.T) {
			pair := testGenerateCert(t)
			pair.Certificate.Raw = make([]byte, 1024)
			err := pool.AddPrivateClientCert(pair.Certificate, pair.PrivateKey)
			require.Error(t, err)
		})

		t.Run("invalid private key", func(t *testing.T) {
			pair := testGenerateCert(t)
			patchFunc := func(_ interface{}) ([]byte, error) {
				return nil, monkey.ErrMonkey
			}
			pg := monkey.Patch(x509.MarshalPKCS8PrivateKey, patchFunc)
			defer pg.Unpatch()
			err := pool.AddPrivateClientCert(pair.Certificate, pair.PrivateKey)
			monkey.IsMonkeyError(t, err)
		})
	})
}
